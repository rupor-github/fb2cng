package kfx

// Fragment represents a single KFX fragment (aka entity payload) before it is
// packed into a CONT container.
//
// In KFXInput terminology, the fragment key is Ion annotations: [fid, ftype].
// Here we keep them as strings and let ion-go resolve them using the document
// symbol table.
type Fragment struct {
	FID   string
	FType string
	Value any
}
