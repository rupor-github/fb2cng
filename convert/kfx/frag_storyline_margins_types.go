package kfx

import "fbc/convert/margins"

// Re-export margin collapsing types for use within the kfx package.
type (
	ContainerKind  = margins.ContainerKind
	ContainerFlags = margins.ContainerFlags
	ContentNode    = margins.ContentNode
	ContentTree    = margins.ContentTree
)

const (
	ContainerRoot       = margins.ContainerRoot
	ContainerSection    = margins.ContainerSection
	ContainerPoem       = margins.ContainerPoem
	ContainerStanza     = margins.ContainerStanza
	ContainerCite       = margins.ContainerCite
	ContainerEpigraph   = margins.ContainerEpigraph
	ContainerFootnote   = margins.ContainerFootnote
	ContainerTitleBlock = margins.ContainerTitleBlock
	ContainerAnnotation = margins.ContainerAnnotation

	FlagTitleBlockMode             = margins.FlagTitleBlockMode
	FlagHasBorderTop               = margins.FlagHasBorderTop
	FlagHasBorderBottom            = margins.FlagHasBorderBottom
	FlagHasPaddingTop              = margins.FlagHasPaddingTop
	FlagHasPaddingBottom           = margins.FlagHasPaddingBottom
	FlagPreventCollapseTop         = margins.FlagPreventCollapseTop
	FlagPreventCollapseBottom      = margins.FlagPreventCollapseBottom
	FlagStripMiddleMarginBottom    = margins.FlagStripMiddleMarginBottom
	FlagTransferMBToLastChild      = margins.FlagTransferMBToLastChild
	FlagForceTransferMBToLastChild = margins.FlagForceTransferMBToLastChild
)
