package kfx

// imageResourceInfo holds image resource information including dimensions.
// Used to create KPV-compatible width-based styles for images.
type imageResourceInfo struct {
	ResourceName string // KFX resource name (e.g., "e1")
	Width        int    // Image width in pixels
	Height       int    // Image height in pixels
}

type imageResourceInfoByID map[string]imageResourceInfo

type sectionNameList []string

type sectionEIDsBySectionName map[string][]int

type eidByFB2ID map[string]int
