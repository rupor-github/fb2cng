// Package pdf contains fb2cng's native fixed-layout PDF renderer.
//
// The renderer is deliberately split into small pipeline stages:
//
//   - blocks.go and blocks_structure.go turn FB2/content model units into a flat
//     stream of pdfTextBlock values. The stream already contains generated pages
//     such as annotation and TOC pages, synthetic backlinks, and references to
//     images or printed footnotes.
//   - styles_*.go builds a resolver from built-in defaults plus book CSS and
//     resolves each block to concrete point-sized typography and box metrics.
//   - layout*.go paginates blocks into pdfPage values. A page is still a logical
//     display list: lines, images, anchors, link annotations, backgrounds, and
//     borders are collected without assigning PDF object numbers yet.
//   - font_*.go, images.go, links.go, outline.go, and metadata.go convert the
//     logical display list into PDF resources.
//   - content_stream.go and docwriter write the final PDF 1.4 object graph.
//
// Coordinates and distances in this package are PDF points unless a field name
// says otherwise. Page coordinates use the PDF convention: origin at the bottom
// left, X grows right, Y grows up. The layout code tracks l.y as the next text
// baseline while walking down from the top content edge.
package pdf
