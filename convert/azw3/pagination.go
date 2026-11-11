package azw3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"

	"github.com/beevik/etree"
	parse "github.com/tdewolff/parse/v2"
	csslex "github.com/tdewolff/parse/v2/css"

	"fbc/css"
)

type paginationDeclaration struct {
	property, value string
	important       bool
}

type paginationPart struct {
	tag, id string
	classes []string
	child   bool
}

type paginationRule struct {
	selector     string
	parts        []paginationPart
	specificity  [3]int
	declarations []paginationDeclaration
}

type paginationPlan struct {
	rules    []paginationRule
	deferred []string
}

// compilePagination only materializes statically decidable CSS breaks. It does
// not split publication documents or choose a new pagination/media policy.
// Unresolved conditions/selectors affecting breaks or visibility remain with the
// reader rather than turning a device-dependent rule into an unconditional break.
func compilePagination(data []byte) *paginationPlan {
	plan := &paginationPlan{}
	selected, err := css.ResolveMedia(data, css.MediaContext{KF8: true}, nil)
	if err != nil {
		plan.deferred = append(plan.deferred, "malformed conditional CSS: "+err.Error())
		return plan
	}
	parser := csslex.NewParser(parse.NewInputBytes(selected), false)
	depth := 0
	var selectors string
	var declarations []paginationDeclaration
	for {
		grammar, _, data := parser.Next()
		switch grammar {
		case csslex.ErrorGrammar:
			if err := parser.Err(); err != nil && !errors.Is(err, io.EOF) {
				plan.deferred = append(plan.deferred, "CSS pagination parse error: "+err.Error())
			}
			return plan
		case csslex.BeginAtRuleGrammar:
			depth++
		case csslex.EndAtRuleGrammar:
			depth--
		case csslex.BeginRulesetGrammar:
			selectors = string(data) + paginationTokens(parser.Values())
			declarations = nil
		case csslex.DeclarationGrammar:
			declaration, relevant, supported := paginationProperty(string(data), paginationTokens(parser.Values()))
			if !relevant {
				continue
			}
			if depth > 0 || !supported {
				plan.deferred = append(plan.deferred, selectors+": "+string(data)+": "+paginationTokens(parser.Values()))
				continue
			}
			declarations = append(declarations, declaration)
		case csslex.EndRulesetGrammar:
			if depth != 0 || len(declarations) == 0 {
				continue
			}
			for selector := range strings.SplitSeq(selectors, ",") {
				rule, supported := parsePaginationSelector(strings.TrimSpace(selector))
				if !supported {
					plan.deferred = append(plan.deferred, "unsupported pagination/visibility selector: "+selector)
					continue
				}
				rule.declarations = slices.Clone(declarations)
				plan.rules = append(plan.rules, rule)
			}
		}
	}
}

func paginationTokens(tokens []csslex.Token) string {
	var text strings.Builder
	for _, token := range tokens {
		if token.TokenType != csslex.CommentToken {
			text.Write(token.Data) // strings.Builder writes cannot fail.
		}
	}
	return text.String()
}

func paginationProperty(property, value string) (paginationDeclaration, bool, bool) {
	property = strings.ToLower(property)
	if property != "page-break-before" && property != "page-break-after" &&
		property != "display" && property != "position" && property != "float" {
		return paginationDeclaration{}, false, true
	}
	value = strings.TrimSpace(strings.ToLower(value))
	important := false
	if at := strings.LastIndexByte(value, '!'); at >= 0 && strings.TrimSpace(value[at+1:]) == "important" {
		important, value = true, strings.TrimSpace(value[:at])
	}
	supported := value == "always" || value == "auto" || value == "avoid"
	if property == "display" {
		supported = slices.Contains([]string{"none", "block", "inline", "inline-block", "table", "list-item", "flex", "grid"}, value)
	} else if property == "position" {
		supported = slices.Contains([]string{"static", "relative", "absolute", "fixed"}, value)
	} else if property == "float" {
		supported = slices.Contains([]string{"none", "left", "right"}, value)
	}
	return paginationDeclaration{property: property, value: value, important: important}, true, supported
}

func parsePaginationSelector(selector string) (paginationRule, bool) {
	rule := paginationRule{selector: selector}
	child := false
	for _, field := range strings.Fields(strings.ReplaceAll(selector, ">", " > ")) {
		if field == ">" {
			if len(rule.parts) == 0 || child {
				return rule, false
			}
			child = true
			continue
		}
		part := paginationPart{child: child}
		child = false
		for len(field) > 0 {
			kind := byte(0)
			if field[0] == '.' || field[0] == '#' {
				kind, field = field[0], field[1:]
			}
			end := strings.IndexAny(field, ".#")
			if end < 0 {
				end = len(field)
			}
			name := field[:end]
			if name == "" {
				return rule, false
			}
			for _, r := range name {
				if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' && !(kind == 0 && name == "*") {
					return rule, false
				}
			}
			switch kind {
			case '.':
				part.classes = append(part.classes, name)
				rule.specificity[1]++
			case '#':
				if part.id != "" {
					return rule, false
				}
				part.id = name
				rule.specificity[0]++
			default:
				part.tag = strings.ToLower(name)
				if name != "*" {
					rule.specificity[2]++
				}
			}
			field = field[end:]
		}
		rule.parts = append(rule.parts, part)
	}
	return rule, len(rule.parts) > 0 && !child
}

func (rule paginationRule) matches(element *etree.Element) bool {
	index := len(rule.parts) - 1
	if !rule.parts[index].matches(element) {
		return false
	}
	for index > 0 {
		child := rule.parts[index].child
		index--
		element = element.Parent()
		for element != nil && !rule.parts[index].matches(element) && !child {
			element = element.Parent()
		}
		if element == nil || !rule.parts[index].matches(element) {
			return false
		}
	}
	return true
}

func (part paginationPart) matches(element *etree.Element) bool {
	if element.Tag == "" {
		return false // The etree document container is not an HTML element.
	}
	if part.tag != "" && part.tag != "*" && !strings.EqualFold(part.tag, element.Tag) ||
		part.id != "" && part.id != element.SelectAttrValue("id", "") {
		return false
	}
	classes := strings.Fields(element.SelectAttrValue("class", ""))
	for _, class := range part.classes {
		if !slices.Contains(classes, class) {
			return false
		}
	}
	return true
}

type paginationWinner struct {
	paginationDeclaration
	selector    string
	specificity [3]int
	inline      bool
}

func (candidate paginationWinner) replaces(previous paginationWinner) bool {
	if previous.property == "" || candidate.important != previous.important {
		return previous.property == "" || candidate.important
	}
	if candidate.inline != previous.inline {
		return candidate.inline
	}
	return slices.Compare(candidate.specificity[:], previous.specificity[:]) >= 0
}

func (plan *paginationPlan) apply(ctx context.Context, doc *pendingDocument) error {
	deferred := slices.Clone(plan.deferred)
	var elements []*etree.Element
	stack := []*etree.Element{doc.Doc.Root()}
	states := make(map[*etree.Element]map[string]paginationWinner)
	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		element := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		elements = append(elements, element)
		children := element.ChildElements()
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, children[i])
		}
		state, unresolved := plan.elementState(element)
		deferred = append(deferred, unresolved...)
		states[element] = state
	}
	if len(deferred) > 0 {
		for _, reason := range deferred {
			doc.Changes = append(doc.Changes, markupChange{Action: "retained CSS pagination for reader", Selector: reason})
		}
		return nil
	}
	for _, element := range elements {
		if err := ctx.Err(); err != nil {
			return err
		}
		if namespace := element.NamespaceURI(); namespace != "" && namespace != "http://www.w3.org/1999/xhtml" {
			continue
		}
		if explicitPageBreak(element) {
			continue // The source already contains the required native representation.
		}
		hidden, outOfFlow := false, false
		for ancestor := element; ancestor != nil; ancestor = ancestor.Parent() {
			hidden = hidden || states[ancestor]["display"].value == "none"
			position, floating := states[ancestor]["position"].value, states[ancestor]["float"].value
			outOfFlow = outOfFlow || position == "absolute" || position == "fixed" || floating == "left" || floating == "right"
		}
		display := states[element]["display"].value
		if hidden || outOfFlow || display == "inline" || display == "inline-block" || (display == "" && !paginationBlock(element.Tag)) {
			for _, property := range []string{"page-break-before", "page-break-after"} {
				if winner := states[element][property]; winner.value == "always" {
					doc.Changes = append(doc.Changes, markupChange{
						Tag: element.Tag, ID: element.SelectAttrValue("id", ""), Classes: element.SelectAttrValue("class", ""),
						Action: "not materialized on hidden, out-of-flow or inline box", Selector: winner.selector,
					})
				}
			}
			continue
		}
		if cssPageBreak(element) && states[element]["page-break-after"].value == "always" {
			parent := element.Parent()
			if parent != nil && paginationContainer(parent.Tag) {
				action := "lowered explicit CSS page break"
				for index, token := range parent.Child {
					if token != element {
						continue
					}
					if adjacentPageBreak(parent, index) {
						action = "coalesced explicit CSS page break"
					} else {
						element.CreateAttr("data-AmznPageBreak", "always")
					}
					break
				}
				element.RemoveAttr("style")
				doc.Changes = append(doc.Changes, markupChange{
					Tag: element.Tag, ID: element.SelectAttrValue("id", ""), Classes: element.SelectAttrValue("class", ""),
					Action: action, Selector: "style attribute",
				})
				continue
			}
		}
		for _, property := range []string{"page-break-before", "page-break-after"} {
			winner := states[element][property]
			if winner.value != "always" {
				continue
			}
			parent := element.Parent()
			if parent == nil || !paginationContainer(parent.Tag) {
				doc.Changes = append(doc.Changes, markupChange{Tag: element.Tag, Action: "retained table/inline-container break for reader"})
				continue
			}
			action := "materialized "
			for index, token := range parent.Child {
				if token != element {
					continue
				}
				if property == "page-break-after" {
					index++
				}
				if adjacentPageBreak(parent, index) {
					action = "coalesced "
				} else {
					marker := etree.NewElement("div")
					// KindleUnpack renders this as a CSS div, but that decoded
					// representation is not the KF8 reader's native break opcode.
					marker.CreateAttr("data-AmznPageBreak", "always")
					marker.CreateText("")
					parent.InsertChildAt(index, marker)
				}
				break
			}
			doc.Changes = append(doc.Changes, markupChange{
				Tag: element.Tag, ID: element.SelectAttrValue("id", ""), Classes: element.SelectAttrValue("class", ""),
				Action: fmt.Sprintf("%s%s:%s", action, property, winner.value), Selector: winner.selector,
			})
		}
	}
	return nil
}

func (plan *paginationPlan) elementState(element *etree.Element) (map[string]paginationWinner, []string) {
	state := make(map[string]paginationWinner)
	var deferred []string
	for _, rule := range plan.rules {
		if rule.matches(element) {
			for _, declaration := range rule.declarations {
				candidate := paginationWinner{paginationDeclaration: declaration, selector: rule.selector, specificity: rule.specificity}
				if candidate.replaces(state[declaration.property]) {
					state[declaration.property] = candidate
				}
			}
		}
	}
	if style := element.SelectAttrValue("style", ""); style != "" {
		parser := csslex.NewParser(parse.NewInputString(style), true)
		for {
			grammar, _, name := parser.Next()
			if grammar == csslex.ErrorGrammar {
				break
			}
			if grammar != csslex.DeclarationGrammar {
				continue
			}
			declaration, relevant, supported := paginationProperty(string(name), paginationTokens(parser.Values()))
			if !relevant {
				continue
			}
			if !supported {
				deferred = append(deferred, "unresolved inline pagination/visibility: "+style)
				continue
			}
			candidate := paginationWinner{paginationDeclaration: declaration, selector: "style attribute", inline: true}
			if candidate.replaces(state[declaration.property]) {
				state[declaration.property] = candidate
			}
		}
	}
	return state, deferred
}

// lowerFootnoteMore resolves the generated indicator before layout is frozen.
// Like KFX, display:none suppresses generation; visible indicators become plain
// text so AZW3 float-note bodies contain no extra formatting elements.
func (plan *paginationPlan) lowerFootnoteMore(doc *pendingDocument) {
	for _, marker := range doc.Doc.FindElements("//span[@data-fbc-footnote-more]") {
		hidden := false
		for element := marker; element != nil; element = element.Parent() {
			state, _ := plan.elementState(element)
			if state["display"].value == "none" {
				hidden = true
				break
			}
		}
		parent := marker.Parent()
		if !hidden {
			for i, token := range parent.Child {
				if token == marker {
					parent.InsertChildAt(i, etree.NewText(marker.Text()))
					break
				}
			}
		}
		parent.RemoveChild(marker)
		doc.Changes = append(doc.Changes, markupChange{Tag: "span", Classes: "footnote-more",
			Action: fmt.Sprintf("lowered generated footnote indicator to plain text (hidden=%t)", hidden)})
	}
}

func explicitPageBreak(element *etree.Element) bool {
	return element.Tag == "div" && strings.EqualFold(element.SelectAttrValue("data-AmznPageBreak", ""), "always") &&
		len(element.ChildElements()) == 0 && strings.TrimSpace(element.Text()) == ""
}

func cssPageBreak(element *etree.Element) bool {
	style := strings.ToLower(strings.Join(strings.Fields(element.SelectAttrValue("style", "")), ""))
	return element.Tag == "div" && strings.TrimSuffix(style, ";") == "page-break-after:always" &&
		len(element.ChildElements()) == 0 && strings.TrimSpace(element.Text()) == ""
}

func adjacentPageBreak(parent *etree.Element, index int) bool {
	for _, direction := range []struct{ start, step int }{{index - 1, -1}, {index, 1}} {
		for i := direction.start; i >= 0 && i < len(parent.Child); i += direction.step {
			switch token := parent.Child[i].(type) {
			case *etree.CharData:
				if strings.TrimSpace(token.Data) == "" {
					continue
				}
			case *etree.Comment:
				continue
			case *etree.Element:
				if explicitPageBreak(token) {
					return true
				}
			}
			break
		}
	}
	return false
}

func paginationBlock(tag string) bool {
	return paginationContainer(tag) || slices.Contains([]string{"p", "h1", "h2", "h3", "h4", "h5", "h6", "table", "pre", "hr"}, tag)
}

func paginationContainer(tag string) bool {
	return slices.Contains([]string{"body", "div", "section", "article", "aside", "blockquote", "li", "dd", "dt", "main", "header", "footer"}, tag)
}
