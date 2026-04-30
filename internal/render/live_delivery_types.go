package render

type DeliveryReport struct {
	SourceDir      string        `json:"source_dir"`
	AlbumName      string        `json:"album_name"`
	CandidatePairs int           `json:"candidate_pairs"`
	SkippedPairs   int           `json:"skipped_pairs"`
	Status         string        `json:"status"`
	Message        string        `json:"message"`
	SkippedItems   []SkippedItem `json:"skipped_items"`
}

type SkippedItem struct {
	BaseName  string `json:"base_name"`
	PhotoPath string `json:"photo_path"`
	VideoPath string `json:"video_path"`
	Message   string `json:"message"`
}

const (
	deliveryStatusPartial = "partial"
	deliveryStatusFailed  = "failed"
)
