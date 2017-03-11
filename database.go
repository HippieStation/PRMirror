package main

// TODO: Make this less bad, all of it

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

func (d *Database) StoreMirror(downstreamID []byte, upstreamID []byte) error {

	// Store the upstream->downstream id
	d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("up2down"))
		err := b.Put([]byte(upstreamID), []byte(downstreamID))
		return err
	})

	// Store the upstream->downstream id
	d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("down2up"))
		err := b.Put([]byte(downstreamID), []byte(upstreamID))
		return err
	})

	return nil
}

func (d *Database) GetDownstreamID(upstreamID []byte) []byte {
	var retval = []byte{0}
	d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("up2down"))
		v := b.Get([]byte(upstreamID))
		copy(retval, v)
		return nil
	})
	return retval
}

func (d *Database) GetUpstreamID(downstreamID []byte) []byte {
	var retval = []byte{0}
	d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("down2up"))
		v := b.Get([]byte(downstreamID))
		copy(retval, v)
		return nil
	})
	return retval
}
