package main

// TODO: Make this less bad, all of it

import (
	"bytes"
	"encoding/binary"

	"fmt"
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

	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("up2down"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})

	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("down2up"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})

	return &Database{db}
}

func (d *Database) Path() string {
	return d.db.Path()
}

func (d *Database) Close() {
	d.db.Close()
}

func (d *Database) IntToByteArray(intIn int) []byte {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, int64(intIn))
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func (d *Database) StoreMirror(downstreamID int, upstreamID int) error {
	downstreamIDBytes := d.IntToByteArray(downstreamID)
	upstreamIDBytes := d.IntToByteArray(upstreamID)

	// Store the upstream->downstream id
	d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("up2down"))
		err := b.Put(upstreamIDBytes, downstreamIDBytes)
		return err
	})

	// Store the upstream->downstream id
	d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("down2up"))
		err := b.Put(downstreamIDBytes, upstreamIDBytes)
		return err
	})

	return nil
}

func (d *Database) GetDownstreamID(upstreamID int) []byte {
	upstreamIDBytes := d.IntToByteArray(upstreamID)

	var retval = []byte{0}
	d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("up2down"))
		v := b.Get(upstreamIDBytes)
		copy(retval, v)
		return nil
	})
	return retval
}

func (d *Database) GetUpstreamID(downstreamID int) []byte {
	downstreamIDBytes := d.IntToByteArray(downstreamID)

	var retval = []byte{0}
	d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("down2up"))
		v := b.Get(downstreamIDBytes)
		copy(retval, v)
		return nil
	})
	return retval
}
