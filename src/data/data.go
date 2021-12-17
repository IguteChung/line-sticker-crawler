package data

// StickerQuery defines the response model for emotion query.
type StickerQuery struct {
	Items []*StickerPackage `json:"items"`
}

// StickerPackage defines a package model for emotion query.
type StickerPackage struct {
	ID string `json:"id"`
}
