package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SingKaraoke/sing-lyrics-pkg/models"
)

// DeviceStore looks up registered client devices (public keys) in Postgres.
type DeviceStore struct {
	pool *pgxpool.Pool
}

func NewDeviceStore(pool *pgxpool.Pool) *DeviceStore {
	return &DeviceStore{pool: pool}
}

// Get returns nil, nil if the device id is unknown (not an error - the caller
// treats "unknown" and "revoked" the same way: request rejected).
func (s *DeviceStore) Get(ctx context.Context, deviceID string) (*models.Device, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, name, public_key, status FROM devices WHERE id = $1`, deviceID)

	var d models.Device
	if err := row.Scan(&d.ID, &d.Name, &d.PublicKey, &d.Status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}

func (s *DeviceStore) Register(ctx context.Context, id, name string, publicKey []byte) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO devices (id, name, public_key, status) VALUES ($1, $2, $3, 'active')`,
		id, name, publicKey)
	return err
}

func (s *DeviceStore) Revoke(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE devices SET status = 'revoked' WHERE id = $1`, id)
	return err
}
