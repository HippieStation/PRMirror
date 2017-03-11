package main

import (
	"github.com/boltdb/bolt"
)

type Database struct {
	db *bolt.DB
}

func NewDatabase() *Database {
	db, err := bolt.Open("mirror.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}

	return &Database{db}
}

func (d *Database) Path() string {
	return d.db.Path()
}

func (d *Database) Close() {
	d.db.Close()
}

func (d *Database) StoreMirror(downstreamID int, upstreamID int) bool {
	return false
}

func (d *Database) GetDownstreamID(upstreamID int) int {
	return 0
}

func (d *Database) GetUpstreamID(downstreamID int) int {
	return 0
}
