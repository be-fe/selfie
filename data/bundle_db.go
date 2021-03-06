package data

import "time"

//Bundle structure for each bundle represents in database
type Bundle struct {
	ID        int64     `db:"id,omitempty,pk" json:"id"`
	ReleaseID int64     `db:"release_id" json:"-"`
	Hash      string    `db:"hash" json:"hash"`
	Name      string    `db:"name" json:"name"`
	Type      FileType  `db:"type" json:"type"`
	CreatedAt time.Time `db:"created_at" json:"created_at" bondb:",utc"`
}

//CollectionName returns collection name in database
func (b *Bundle) CollectionName() string {
	return `bundles`
}
