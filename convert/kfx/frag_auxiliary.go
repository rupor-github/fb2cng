package kfx

// BuildAuxiliaryDataFragments creates $597 auxiliary_data fragments.
// Reference files use these to mark target sections and related synthetic IDs.
func BuildAuxiliaryDataFragments(ids sectionNameList) []*Fragment {
	out := make([]*Fragment, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		// KFXInput requires fragment IDs (the fid symbol) to be unique across all fragment types.
		// Reference files use a separate auxiliary id (often suffixed) while storing the real target
		// in $598 (kfx_id).
		fid := id + "-ad"
		out = append(out, &Fragment{
			FType:   SymAuxiliaryData,
			FIDName: fid,
			Value: NewStruct().
				SetList(SymMetadata, []any{NewMetadataEntry("IS_TARGET_SECTION", true)}).
				SetString(SymKfxID, fid), // $598 = kfx_id as string
		})
	}
	return out
}
