package azw3

import (
	"bytes"
	"container/heap"
	"context"
	"encoding/binary"
	"errors"
	"math"
	"slices"
)

const (
	huffMaxPhrases = 4096
	huffPhraseSize = 32
	huffCodeLimit  = 24 // Fits HUFF's 24-bit fast-table maximum-code field.
	cdicBits       = 10
)

type huffPhrase struct {
	Data      []byte
	Frequency uint64
}

type huffCode struct {
	Bits   uint32
	Length uint8
}

type huffTrieNode struct {
	Symbol   int
	Children map[byte]*huffTrieNode
}

type huffTrie [256]*huffTrieNode

func newHuffTrie(phrases []huffPhrase) *huffTrie {
	tree := new(huffTrie)
	for i := range 256 {
		tree[i] = &huffTrieNode{Symbol: i}
	}
	for i := 256; i < len(phrases); i++ {
		phrase := phrases[i].Data
		node := tree[phrase[0]]
		for _, value := range phrase[1:] {
			if node.Children == nil {
				node.Children = make(map[byte]*huffTrieNode)
			}
			child := node.Children[value]
			if child == nil {
				child = &huffTrieNode{Symbol: -1}
				node.Children[value] = child
			}
			node = child
		}
		node.Symbol = i
	}
	return tree
}

func (tree *huffTrie) match(data []byte) (symbol, size int) {
	node := tree[data[0]]
	symbol, size = node.Symbol, 1
	for i := 1; i < min(len(data), huffPhraseSize); i++ {
		node = node.Children[data[i]]
		if node == nil {
			break
		}
		if node.Symbol >= 0 {
			symbol, size = node.Symbol, i+1
		}
	}
	return symbol, size
}

type huffDictionary struct {
	Tree    *huffTrie
	Codes   []huffCode
	Records [][]byte
}

func buildHuffDictionary(ctx context.Context, chunks []textRecord) (*huffDictionary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	phrases, err := trainHuffPhrases(ctx, chunks)
	if err != nil {
		return nil, err
	}
	// Discard phrases whose actual longest-match usage cannot repay their CDIC
	// storage. Recount after pruning because a removed phrase exposes shorter ones.
	for range 2 {
		if err := countHuffPhrases(ctx, chunks, newHuffTrie(phrases), phrases); err != nil {
			return nil, err
		}
		kept := slices.Clone(phrases[:256])
		for _, phrase := range phrases[256:] {
			if phrase.Frequency > 1 && phrase.Frequency*uint64(len(phrase.Data)-1) > uint64(len(phrase.Data)+4) {
				kept = append(kept, phrase)
			}
		}
		if len(kept) == len(phrases) {
			break
		}
		phrases = kept
	}
	tree := newHuffTrie(phrases)
	if err := countHuffPhrases(ctx, chunks, tree, phrases); err != nil {
		return nil, err
	}
	lengths := huffCodeLengths(phrases)
	order := make([]int, len(phrases))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(a, b int) int {
		if lengths[a] != lengths[b] {
			return int(lengths[a]) - int(lengths[b])
		}
		return bytes.Compare(phrases[a].Data, phrases[b].Data)
	})
	// HUFF uses reverse canonical codes: shortest codes occupy the high end,
	// while CDIC indexes increase as codes decrease within each length class.
	codes := make([]huffCode, len(phrases))
	cursor := uint64(1) << 32
	for _, symbol := range order {
		length := lengths[symbol]
		cursor -= uint64(1) << (32 - length)
		codes[symbol] = huffCode{Bits: uint32(cursor >> (32 - length)), Length: length}
	}
	records, err := encodeHuffRecords(phrases, codes, order)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &huffDictionary{Tree: tree, Codes: codes, Records: records}, nil
}

func (dictionary *huffDictionary) compress(data []byte) []byte {
	result := make([]byte, 0, len(data))
	var pending uint64
	var count uint8
	for pos := 0; pos < len(data); {
		symbol, size := dictionary.Tree.match(data[pos:])
		code := dictionary.Codes[symbol]
		pending = pending<<code.Length | uint64(code.Bits)
		count += code.Length
		for count >= 8 {
			count -= 8
			result = append(result, byte(pending>>count))
		}
		pending &= (uint64(1) << count) - 1
		pos += size
	}
	if count > 0 {
		// All 256 byte literals exist, including unused ones. The all-zero code
		// is therefore at least eight bits long; <=7 zero padding bits cannot
		// decode to another symbol. Padding is not an implicit empty CDIC phrase.
		result = append(result, byte(pending<<(8-count)))
	}
	return result
}

func countHuffPhrases(ctx context.Context, chunks []textRecord, tree *huffTrie, phrases []huffPhrase) error {
	for i := range phrases {
		phrases[i].Frequency = 0
	}
	for _, chunk := range chunks {
		if err := ctx.Err(); err != nil {
			return err
		}
		for pos := 0; pos < len(chunk.Data); {
			symbol, size := tree.match(chunk.Data[pos:])
			phrases[symbol].Frequency++
			pos += size
		}
	}
	return nil
}

// Train on at most 512 KiB of evenly sampled records, using bounded LZ searches
// and a capped candidate map. Training cost/memory do not grow with book length.
// Actual symbol frequencies are counted over all records after phrase selection.
func trainHuffPhrases(ctx context.Context, chunks []textRecord) ([]huffPhrase, error) {
	const sampleRecords, candidateLimit, hashSize = 128, 32768, 8192
	var sample []byte
	n := min(len(chunks), sampleRecords)
	for i := range n {
		index := 0
		if n > 1 {
			index = i * (len(chunks) - 1) / (n - 1)
		}
		sample = append(sample, chunks[index].Data...)
	}
	type key struct {
		Data [huffPhraseSize]byte
		Size uint8
	}
	counts := make(map[key]uint32)
	var heads [hashSize]int
	previous := make([]int, len(sample))
	for pos := 0; pos+2 < len(sample); pos++ {
		if pos%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		hash := int((uint32(sample[pos])*251+uint32(sample[pos+1]))*251+uint32(sample[pos+2])) & (hashSize - 1)
		candidate, longest := heads[hash]-1, 0
		limit := min(huffPhraseSize, len(sample)-pos)
		for attempt := 0; candidate >= 0 && pos-candidate <= math.MaxUint16 && attempt < 8; attempt++ {
			length := 0
			for length < limit && sample[candidate+length] == sample[pos+length] {
				length++
			}
			longest = max(longest, length)
			if longest == limit {
				break
			}
			candidate = previous[candidate] - 1
		}
		previous[pos], heads[hash] = heads[hash], pos+1
		for _, size := range [...]int{3, 4, 6, 8, 12, 16, 24, 32} {
			if size > longest {
				break
			}
			value := key{Size: uint8(size)}
			copy(value.Data[:], sample[pos:pos+size])
			if count := counts[value]; count > 0 || len(counts) < candidateLimit {
				counts[value] = count + 1
			}
		}
	}
	type candidate struct {
		Data  []byte
		Score int64
	}
	var candidates []candidate
	for value, count := range counts {
		score := int64(count)*int64(value.Size-1) - int64(value.Size) - 4
		if count > 1 && score > 0 {
			candidates = append(candidates, candidate{Data: bytes.Clone(value.Data[:value.Size]), Score: score})
		}
	}
	slices.SortFunc(candidates, func(a, b candidate) int {
		if a.Score > b.Score {
			return -1
		}
		if a.Score < b.Score {
			return 1
		}
		return bytes.Compare(a.Data, b.Data)
	})
	phrases := make([]huffPhrase, 256, huffMaxPhrases)
	for i := range 256 {
		phrases[i].Data = []byte{byte(i)}
	}
	for _, candidate := range candidates[:min(len(candidates), huffMaxPhrases-256)] {
		phrases = append(phrases, huffPhrase{Data: candidate.Data})
	}
	return phrases, nil
}

type huffTreeNode struct {
	Weight      uint64
	Left, Right int
	MinSymbol   int
}

type huffQueue struct {
	Items []int
	Nodes []huffTreeNode
}

func (q *huffQueue) Len() int { return len(q.Items) }

func (q *huffQueue) Less(i, j int) bool {
	a, b := q.Nodes[q.Items[i]], q.Nodes[q.Items[j]]
	if a.Weight != b.Weight {
		return a.Weight < b.Weight
	}
	return a.MinSymbol < b.MinSymbol
}

func (q *huffQueue) Swap(i, j int) { q.Items[i], q.Items[j] = q.Items[j], q.Items[i] }

func (q *huffQueue) Push(x any) { q.Items = append(q.Items, x.(int)) }

func (q *huffQueue) Pop() any {
	i := q.Items[len(q.Items)-1]
	q.Items = q.Items[:len(q.Items)-1]
	return i
}

func huffCodeLengths(phrases []huffPhrase) []uint8 {
	// Rare literals need positive weight. If a pathological distribution exceeds
	// the wire's length bound, reduce weight precision and rebuild, rather than
	// truncate codes or create an incomplete prefix tree.
	for shift := uint(0); ; shift++ {
		q := &huffQueue{Nodes: make([]huffTreeNode, len(phrases)), Items: make([]int, len(phrases))}
		for i, phrase := range phrases {
			q.Nodes[i] = huffTreeNode{Weight: 1 + (phrase.Frequency >> shift), Left: -1, Right: -1, MinSymbol: i}
			q.Items[i] = i
		}
		heap.Init(q)
		for q.Len() > 1 {
			a, b := heap.Pop(q).(int), heap.Pop(q).(int)
			index := len(q.Nodes)
			q.Nodes = append(q.Nodes, huffTreeNode{
				Weight: q.Nodes[a].Weight + q.Nodes[b].Weight, Left: a, Right: b,
				MinSymbol: min(q.Nodes[a].MinSymbol, q.Nodes[b].MinSymbol),
			})
			heap.Push(q, index)
		}
		lengths := make([]uint8, len(phrases))
		var longest uint8
		var visit func(int, uint8)
		visit = func(index int, depth uint8) {
			node := q.Nodes[index]
			if node.Left < 0 {
				lengths[index] = depth
				longest = max(longest, depth)
				return
			}
			visit(node.Left, depth+1)
			visit(node.Right, depth+1)
		}
		visit(q.Items[0], 0)
		if longest <= huffCodeLimit {
			return lengths
		}
	}
}

func encodeHuffRecords(phrases []huffPhrase, codes []huffCode, order []int) ([][]byte, error) {
	var minimum, maximum, top, count [33]uint32
	for index, symbol := range order {
		code := codes[symbol]
		if count[code.Length] == 0 {
			top[code.Length] = code.Bits
			maximum[code.Length] = code.Bits + uint32(index)
		}
		minimum[code.Length] = code.Bits
		count[code.Length]++
	}
	const fastOffset, lengthOffset, littleFastOffset, littleLengthOffset = 24, 1048, 1304, 2328
	huff := make([]byte, 2584)
	copy(huff, "HUFF")
	for i, word := range []uint32{24, fastOffset, lengthOffset, littleFastOffset, littleLengthOffset} {
		binary.BigEndian.PutUint32(huff[4+4*i:], word)
	}
	for prefix := range 256 {
		fallback := 9
		for fallback < huffCodeLimit && count[fallback] == 0 {
			fallback++
		}
		value := uint32(fallback) // Begin at an occupied length class, not an unused early slot.
		for length := uint8(1); length <= 8; length++ {
			code := uint32(prefix) >> (8 - length)
			if count[length] > 0 && code >= minimum[length] && code <= top[length] {
				value = maximum[length]<<8 | 0x80 | uint32(length)
				break
			}
		}
		binary.BigEndian.PutUint32(huff[fastOffset+4*prefix:], value)
		binary.LittleEndian.PutUint32(huff[littleFastOffset+4*prefix:], value)
	}
	boundary := uint64(1) << 32
	for length := 1; length <= 32; length++ {
		minCode, maxCode := minimum[length], maximum[length]
		if count[length] == 0 {
			// Empty intermediate classes must still reject every remaining code.
			// Use the unallocated prefix boundary, not 2^length (which would
			// overflow a 32-bit decoder after its normalization shift).
			if boundary < uint64(1)<<32 {
				minCode = uint32(boundary >> (32 - length))
			}
		} else {
			boundary = uint64(minCode) << (32 - length)
		}
		for i, value := range []uint32{minCode, maxCode} {
			offset := 8*(length-1) + 4*i
			binary.BigEndian.PutUint32(huff[lengthOffset+offset:], value)
			binary.LittleEndian.PutUint32(huff[littleLengthOffset+offset:], value)
		}
	}
	records := [][]byte{huff}
	for start := 0; start < len(order); start += 1 << cdicBits {
		n := min(1<<cdicBits, len(order)-start)
		data := make([]byte, 16+2*n)
		copy(data, "CDIC")
		binary.BigEndian.PutUint32(data[4:], 16)
		binary.BigEndian.PutUint32(data[8:], uint32(len(order)))
		binary.BigEndian.PutUint32(data[12:], cdicBits)
		for i, symbol := range order[start : start+n] {
			phrase := phrases[symbol].Data
			if len(phrase) == 0 || len(phrase) > 0x7fff || len(data)-16 > math.MaxUint16 {
				return nil, errors.New("HUFF/CDIC dictionary phrase exceeds record bounds")
			}
			binary.BigEndian.PutUint16(data[16+2*i:], uint16(len(data)-16))
			data = binary.BigEndian.AppendUint16(data, uint16(len(phrase))|0x8000) // Terminal, uncompressed phrase.
			data = append(data, phrase...)
		}
		records = append(records, align4(data))
	}
	return records, nil
}
