// Package models holds domain types shared by every service. IDs are plain
// strings (Postgres uuid columns are read back as text, e.g. "id::text") to
// avoid pulling in a uuid library just to pass values through JSON/SQL.
package models

import "time"

type Artist struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	SortName string `json:"sortName,omitempty"`
}

type Album struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	SortTitle     string     `json:"sortTitle,omitempty"`
	AlbumArtistID string     `json:"albumArtistId,omitempty"`
	ReleaseDate   *time.Time `json:"releaseDate,omitempty"`
	ArtworkURL    string     `json:"artworkUrl,omitempty"`
	Compilation   bool       `json:"compilation"`
}

type Track struct {
	ID              string     `json:"id"`
	ISRC            string     `json:"isrc"`
	AlbumID         string     `json:"albumId,omitempty"`
	PrimaryArtistID string     `json:"primaryArtistId,omitempty"`
	ArtistName      string     `json:"artistName,omitempty"`
	AlbumTitle      string     `json:"albumTitle,omitempty"`
	Title           string     `json:"title"`
	TitleSort       string     `json:"titleSort,omitempty"`
	TrackNumber     int        `json:"trackNumber,omitempty"`
	DiscNumber      int        `json:"discNumber,omitempty"`
	DurationMs      int        `json:"durationMs,omitempty"`
	Explicit        bool       `json:"explicit"`
	Gapless         bool       `json:"gapless"`
	MediaType       string     `json:"mediaType,omitempty"`
	ReleaseDate     *time.Time `json:"releaseDate,omitempty"`
	Genres          []string   `json:"genres,omitempty"`
	HasLyrics       bool       `json:"hasLyrics"`
	WordSynced      bool       `json:"wordSynced"`
	LineSynced      bool       `json:"lineSynced"`
}

type Lyrics struct {
	ID             string    `json:"id"`
	TrackID        string    `json:"trackId"`
	Format         string    `json:"format"`
	Language       string    `json:"language"`
	WordSynced     bool      `json:"wordSynced"`
	LineSynced     bool      `json:"lineSynced"`
	ContentHash    string    `json:"contentHash,omitempty"`
	StorageURL     string    `json:"storageUrl,omitempty"`
	StorageAccount string    `json:"storageAccount,omitempty"` // which configured R2 account (pkg/config.R2Account.Name) StorageURL lives in
	SizeBytes      int       `json:"sizeBytes,omitempty"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"createdAt"`
}

type Device struct {
	ID        string
	Name      string
	PublicKey []byte // raw 32-byte Ed25519 public key
	Status    string // active | revoked
}

func (d *Device) Active() bool { return d.Status == "active" }
