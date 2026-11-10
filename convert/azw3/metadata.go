package azw3

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"slices"
	"strings"
	"unicode/utf8"
	"uuid"

	"github.com/beevik/etree"
	"github.com/disintegration/imaging"
	"go.uber.org/zap"
	"golang.org/x/text/encoding/charmap"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/convert/metainfo"
	"fbc/convert/publication"
	"fbc/fb2"
	"fbc/misc"
	imgutil "fbc/utils/images"
)

type metadataLayout struct {
	Title       string
	Database    string
	UID         uint32
	Language    uint32
	ASIN        string
	ContentType string
	HasFonts    bool
	EXTH        []exthRecord
	Resources   [][]byte
}

// compileMetadata extends a private resource list without renumbering any image,
// font or SVG reference frozen by the text pass. Cover/thumbnail offsets and RESC
// addresses are relative to FirstResource, not absolute PalmDB record numbers.
func compileMetadata(
	ctx context.Context, c *content.Content, cfg *config.DocumentConfig,
	pub *publication.Publication, text *textLayout, navigation *navigationLayout, log *zap.Logger,
) (*metadataLayout, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c == nil || c.Book == nil || cfg == nil || pub == nil || text == nil {
		return nil, errors.New("prepared content, configuration, publication and text layout are required")
	}
	if _, err := uuid.Parse(c.Book.Description.DocumentInfo.ID); err != nil {
		return nil, fmt.Errorf("prepared book identifier is not a UUID: %w", err)
	}
	asin, err := common.NormalizeASIN(c.ASIN)
	if err != nil {
		return nil, fmt.Errorf("invalid Kindle ASIN override: %w", err)
	}
	digest := sha256.Sum256([]byte(c.Book.Description.DocumentInfo.ID))
	catalogIdentity := asin != "" || c.KindleEbook
	if asin == "" && catalogIdentity {
		// Explicit EBOK output still needs a deterministic content identifier.
		const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		var generated [sha256.Size]byte
		for i, value := range digest {
			generated[i] = alphabet[int(value)%len(alphabet)]
		}
		asin = string(generated[:])
	}
	title := c.Book.Description.TitleInfo.BookTitle.Value
	if cfg.Metainformation.TitleTemplate != "" {
		expanded, err := c.Book.ExpandTemplateMetainfo(
			config.MetaTitleTemplateFieldName, cfg.Metainformation.TitleTemplate, c.SrcName, c.OutputFormat,
		)
		if err != nil {
			log.Warn("Unable to expand title template for AZW3 metadata", zap.Error(err))
		} else {
			title = expanded
		}
	}
	if cfg.Metainformation.Transliterate {
		title = fb2.Transliterate(title)
	}
	if title == "" || !utf8.ValidString(title) {
		return nil, errors.New("AZW3 metadata title must be nonempty UTF-8")
	}
	result := &metadataLayout{
		Title: title, Database: databaseName(title), UID: binary.BigEndian.Uint32(digest[:4]),
		Language: mobiLanguage(c.Book.Description.TitleInfo.Lang), ASIN: asin,
		Resources: slices.Clone(text.Resources),
	}
	if catalogIdentity {
		result.ContentType = "PDOC"
		if c.KindleEbook {
			result.ContentType = "EBOK"
		}
	} else {
		// Like local KindleGen output, leave catalog ASIN/type fields absent.
		// Synthetic catalog metadata makes Voyage ignore the embedded library
		// thumbnail. UID/RESC retain the book identity; local APNX uses no ASIN.
		log.Debug("Using local AZW3 metadata without catalog identifiers")
	}
	var textErr error
	addText := func(id uint32, value string) {
		if !utf8.ValidString(value) && textErr == nil {
			textErr = fmt.Errorf("EXTH field %d must be UTF-8", id)
		}
		if value != "" {
			result.EXTH = append(result.EXTH, exthRecord{ID: id, Data: []byte(value)})
		}
	}
	addNumber := func(id, value uint32) {
		result.EXTH = append(result.EXTH, exthRecord{ID: id, Data: binary.BigEndian.AppendUint32(nil, value)})
	}
	for i := range c.Book.Description.TitleInfo.Authors {
		name, err := metainfo.PersonName(c.Book, &cfg.Metainformation, c.OutputFormat, i, &c.Book.Description.TitleInfo.Authors[i])
		if err != nil {
			log.Warn("Unable to expand author name template for AZW3 metadata", zap.Error(err))
		}
		addText(100, name)
	}
	addText(101, metainfo.Publisher(c.Book.Description.PublishInfo))
	if annotation := c.Book.Description.TitleInfo.Annotation; annotation != nil {
		addText(103, annotation.AsPlainText())
	}
	if isbn, _, err := metainfo.ISBN(c.Book.Description.PublishInfo); err != nil {
		log.Debug("Invalid ISBN metadata, skipping", zap.Error(err))
	} else {
		addText(104, isbn)
	}
	for _, subject := range metainfo.Subjects(c.Book.Description.TitleInfo.Genres, c.Book.Description.TitleInfo.Keywords) {
		addText(105, subject.Value)
	}
	if date, err := metainfo.KindleIssueDate(&c.Book.Description); err != nil {
		log.Debug("Invalid date metadata, skipping", zap.Error(err))
	} else {
		addText(106, date)
	}
	var translators []string
	for i := range c.Book.Description.TitleInfo.Translators {
		name, err := metainfo.PersonName(c.Book, &cfg.Metainformation, c.OutputFormat, i, &c.Book.Description.TitleInfo.Translators[i])
		if err != nil {
			log.Warn("Unable to expand translator name template for AZW3 metadata", zap.Error(err))
		}
		if name != "" {
			addText(108, name)
			translators = append(translators, name)
		}
	}
	addText(113, asin)
	addText(501, result.ContentType)
	addText(503, title)
	addText(504, asin)
	addText(524, c.Book.Description.TitleInfo.Lang.String())
	addText(535, misc.GetAppName()+"/"+misc.GetVersion())
	if navigation != nil && navigation.Start != nil {
		if navigation.Start.Position >= text.MainLength {
			return nil, errors.New("start-reading offset lies outside frozen main text")
		}
		addNumber(116, navigation.Start.Position)
	}
	for _, reference := range text.References {
		result.HasFonts = result.HasFonts || reference.Kind == resourceFont
	}
	virtualCover := pub.CoverID != ""
	for document := range pub.Documents() {
		if document.Filename == "cover.xhtml" {
			virtualCover = false // The retained SVG fallback has a real cover skeleton.
		}
	}
	resc, err := encodeMetadataResource(
		c.Book.Description.DocumentInfo.ID, translators, c.Book.Description.TitleInfo.Sequences, text, virtualCover,
	)
	if err != nil {
		return nil, err
	}
	metadataOffset := uint32(len(result.Resources))
	result.Resources = append(result.Resources, resc)
	// 129 names RESC, not the thumbnail. 131 is its zero-based resource offset.
	addText(129, "kindle:embed:"+base32(metadataOffset+1, 4))
	addNumber(131, metadataOffset)
	if pub.CoverID != "" {
		cover := pub.Images[pub.CoverID]
		if cover == nil {
			return nil, errors.New("publication cover is missing from the image inventory")
		}
		reference, found := text.References[cover.Filename]
		if found && reference.Kind == resourceSVG && cfg.Images.UseBroken {
			// A failed cover rasterization retained the original SVG. Its flow and
			// cover page remain available, but EXTH raster offsets would be invalid.
			log.Warn("Retained SVG cover cannot supply AZW3 raster cover/thumbnail metadata")
		} else if !found || reference.Kind != resourceImage || reference.Number == 0 || int(reference.Number) > len(text.Resources) ||
			!bytes.Equal(text.Resources[reference.Number-1], cover.Data) {
			return nil, errors.New("publication cover has no matching frozen raster resource")
		} else {
			thumb, err := coverThumbnail(cover.Data)
			if err != nil {
				// use_broken can retain undecodable cover bytes. Do not replace them or
				// turn the original warning-only image policy into a new fatal error.
				log.Warn("Unable to create AZW3 cover thumbnail; using original cover resource", zap.Error(err))
				addNumber(202, reference.Number-1)
			} else {
				addNumber(202, uint32(len(result.Resources)))
				result.Resources = append(result.Resources, thumb)
			}
			addNumber(201, reference.Number-1)
			addNumber(203, 0)
		}
	}
	addNumber(125, uint32(len(result.Resources)))
	if textErr != nil {
		return nil, textErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func coverThumbnail(data []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	thumbnail := imaging.Thumbnail(img, 330, 470, imaging.Lanczos)
	return imgutil.EncodeJPEGWithDPI(thumbnail, 75, imgutil.DpiPxPerInch, 300, 300)
}

// databaseName reproduces KindleGen's single-byte PalmDB identifier, not the
// Unicode display title. Keep its Unicode representation for JSON/APNX; the
// PalmDB writer encodes the field as Windows-1252.
func databaseName(title string) string {
	// KindleGen de-accents these Latin-1 values, retains some single-byte
	// letters and drops multiplication/division signs. It does not normalize
	// combining sequences or transliterate scripts above the single-byte range.
	const accents = "AAAAAA\xc6CEEEEIIII\xd0NOOOOO\x00\xd8UUUUY\xde\xdf" +
		"aaaaaa\xe6ceeeeiiii\xf0nooooo\x00\xf8uuuuy\xfey"
	var encoded []byte
	for _, char := range strings.Trim(title, " \t\r\n\v\f\u00a0") {
		value := uint16(char) // KindleGen narrows even supplementary code points.
		var next byte
		switch {
		case value == 0x100:
			next = 'A' // Measured KindleGen exception at the single-byte boundary.
		case value > 0xff:
			next = 'A' + byte(value%26)
		case value >= 'a' && value <= 'z', value >= 'A' && value <= 'Z', value >= '0' && value <= '9',
			value == '-', value == '_', value == '.':
			next = byte(value)
		case value == ' ':
			next = '_'
		case value >= 0xc0:
			next = accents[value-0xc0]
		case value == 0x83, value == 0x8c, value == 0x9c:
			next = byte(value)
		case value == 0x8a:
			next = 'S'
		case value == 0x8e:
			next = 'Z'
		case value == 0x9a:
			next = 's'
		case value == 0x9e:
			next = 'z'
		case value == 0x9f:
			next = 'Y'
		case value == 0xaa:
			next = 'a'
		case value == 0xb5:
			next = 0xbc
		case value == 0xba:
			next = 'o'
		}
		if next != 0 {
			encoded = append(encoded, next)
		}
	}
	if len(encoded) > 27 {
		encoded = append(append(bytes.Clone(encoded[:12]), '-'), encoded[len(encoded)-14:]...)
	}
	var result strings.Builder
	for _, value := range encoded {
		result.WriteRune(charmap.Windows1252.DecodeByte(value))
	}
	return result.String()
}

func encodeMetadataResource(
	bookID string, translators []string, sequences []fb2.Sequence, text *textLayout, virtualCover bool,
) ([]byte, error) {
	doc := etree.NewDocument()
	doc.CreateProcInst("xml", `version="1.0" encoding="UTF-8"`)
	pkg := doc.CreateElement("package")
	pkg.CreateAttr("xmlns", "http://www.idpf.org/2007/opf")
	pkg.CreateAttr("version", "3.0")
	meta := pkg.CreateElement("metadata")
	meta.CreateAttr("xmlns:dc", "http://purl.org/dc/elements/1.1/")
	meta.CreateElement("dc:identifier").SetText(bookID)
	for i, name := range translators {
		id := fmt.Sprintf("translator%d", i)
		person := meta.CreateElement("dc:contributor")
		person.CreateAttr("id", id)
		person.SetText(name)
		role := meta.CreateElement("meta")
		role.CreateAttr("refines", "#"+id)
		role.CreateAttr("property", "role")
		role.CreateAttr("scheme", "marc:relators")
		role.SetText("trl")
	}
	metainfo.AddSequences(meta, sequences)
	spine := pkg.CreateElement("spine")
	if virtualCover {
		// Like KindleGen's suppressed source cover, this entry has no skelid.
		// EXTH cover/thumbnail offsets supply its image outside the main flow.
		item := spine.CreateElement("itemref")
		item.CreateAttr("idref", "fbc-cover")
		item.CreateAttr("linear", "no")
	}
	for i := range text.Documents {
		item := spine.CreateElement("itemref")
		item.CreateAttr("idref", fmt.Sprintf("fbc-part-%d", i))
		item.CreateAttr("skelid", fmt.Sprint(i))
	}
	xml, err := doc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("serialize KF8 extra metadata: %w", err)
	}
	if !utf8.Valid(xml) || uint64(len(xml)) > uint64(^uint32(0)) {
		return nil, errors.New("invalid or oversized KF8 extra metadata")
	}
	header := "size=" + base32(uint32(len(xml)), 0) + "&version=1&type=1"
	data := make([]byte, 16)
	copy(data, "RESC")
	binary.BigEndian.PutUint32(data[4:], 16)
	binary.BigEndian.PutUint32(data[8:], 1)
	binary.BigEndian.PutUint32(data[12:], uint32(len(header)))
	data = append(data, header...)
	return append(data, xml...), nil
}
