package kfx

import "fbc/convert/margins"

// CollapseMargins applies CSS margin collapsing as a post-processing step.
// This is called after all content is generated and margins are captured.
func (sb *StorylineBuilder) CollapseMargins() *ContentTree {
	tree := sb.buildContentTree()
	margins.CollapseTree(tree)
	return tree
}
