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

func (b *Database) Path() string {
	return b.db.Path()
}

func (b *Database) Close() {
	b.db.Close()
}