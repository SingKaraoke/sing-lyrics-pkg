// Package search wraps the Meilisearch client. Meilisearch is a read
// projection, never a source of truth (see docs/ARCHITECTURE.md §1b): if its
// index is lost, it can always be rebuilt from Postgres via tools/backfill-search.
package search

import (
	"context"

	"github.com/meilisearch/meilisearch-go"

	"github.com/SingKaraoke/sing-lyrics-pkg/models"
)

const IndexName = "tracks"

type Client struct {
	sm meilisearch.ServiceManager
}

func NewClient(url, apiKey string) *Client {
	return &Client{sm: meilisearch.New(url, meilisearch.WithAPIKey(apiKey))}
}

// EnsureIndex sets up the index and its search/filter settings. Safe to call
// on every service startup - Meilisearch treats it as an idempotent upsert.
func (c *Client) EnsureIndex(ctx context.Context) error {
	_, err := c.sm.CreateIndexWithContext(ctx, &meilisearch.IndexConfig{
		Uid:        IndexName,
		PrimaryKey: "id",
	})
	if err != nil && !isIndexAlreadyExists(err) {
		return err
	}

	idx := c.sm.Index(IndexName)
	searchable := []string{"title", "artistName", "albumTitle", "isrc"}
	filterable := []interface{}{"genres", "hasLyrics", "wordSynced", "lineSynced", "explicit"}
	if _, err := idx.UpdateSearchableAttributesWithContext(ctx, &searchable); err != nil {
		return err
	}
	if _, err := idx.UpdateFilterableAttributesWithContext(ctx, &filterable); err != nil {
		return err
	}
	return nil
}

func isIndexAlreadyExists(err error) bool {
	apiErr, ok := err.(*meilisearch.Error)
	return ok && apiErr.MeilisearchApiError.Code == "index_already_exists"
}

// TrackDocument is the flat, denormalized shape indexed for search - the
// join of tracks/artists/albums that Postgres does for a single lookup, done
// once at index time instead of on every search request.
type TrackDocument struct {
	ID         string   `json:"id"`
	ISRC       string   `json:"isrc"`
	Title      string   `json:"title"`
	ArtistName string   `json:"artistName"`
	AlbumTitle string   `json:"albumTitle,omitempty"`
	Genres     []string `json:"genres,omitempty"`
	DurationMs int      `json:"durationMs,omitempty"`
	HasLyrics  bool     `json:"hasLyrics"`
	WordSynced bool     `json:"wordSynced"`
	LineSynced bool     `json:"lineSynced"`
	Explicit   bool     `json:"explicit"`
}

func TrackDocumentFrom(t *models.Track) TrackDocument {
	return TrackDocument{
		ID:         t.ID,
		ISRC:       t.ISRC,
		Title:      t.Title,
		ArtistName: t.ArtistName,
		AlbumTitle: t.AlbumTitle,
		Genres:     t.Genres,
		DurationMs: t.DurationMs,
		HasLyrics:  t.HasLyrics,
		WordSynced: t.WordSynced,
		LineSynced: t.LineSynced,
		Explicit:   t.Explicit,
	}
}

func (c *Client) UpsertTrack(ctx context.Context, doc TrackDocument) error {
	_, err := c.sm.Index(IndexName).AddDocumentsWithContext(ctx, []TrackDocument{doc}, nil)
	return err
}

func (c *Client) UpsertTracks(ctx context.Context, docs []TrackDocument) error {
	_, err := c.sm.Index(IndexName).AddDocumentsWithContext(ctx, docs, nil)
	return err
}

// MarkLyrics does a partial-merge update (Meilisearch's "update documents"
// endpoint, not "add/replace") so it only ever touches the lyrics-related
// fields, never clobbering the rest of a document that upload-api didn't fetch.
func (c *Client) MarkLyrics(ctx context.Context, trackID string, wordSynced, lineSynced bool) error {
	patch := []map[string]interface{}{{
		"id":         trackID,
		"hasLyrics":  true,
		"wordSynced": wordSynced,
		"lineSynced": lineSynced,
	}}
	_, err := c.sm.Index(IndexName).UpdateDocumentsWithContext(ctx, patch, nil)
	return err
}

func (c *Client) DeleteTrack(ctx context.Context, trackID string) error {
	_, err := c.sm.Index(IndexName).DeleteDocumentWithContext(ctx, trackID, nil)
	return err
}

func (c *Client) Search(ctx context.Context, query string, limit int64) ([]TrackDocument, error) {
	res, err := c.sm.Index(IndexName).SearchWithContext(ctx, query, &meilisearch.SearchRequest{Limit: limit})
	if err != nil {
		return nil, err
	}

	var docs []TrackDocument
	if err := res.Hits.Decode(&docs); err != nil {
		return nil, err
	}
	return docs, nil
}
