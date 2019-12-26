package main

// TODO: Make this less bad, all of it

import (
	"encoding/binary"
	"strconv"

	"fmt"

	"github.com/boltdb/bolt"
)

// Database type
type Database struct {
	db *bolt.DB
}

// NewDatabase creates a new DB
func NewDatabase() *Database {
	db, err := bolt.Open("mirror.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}

	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("events"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})

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

// DumpDB dumps DB to stdout
func (d *Database) DumpDB() {
	log.Debugf("down2up")
	d.db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte("down2up"))

		b.ForEach(func(k, v []byte) error {
			log.Debugf("key=%d, value=%d\n", d.btoi(k), d.btoi(v))
			return nil
		})
		return nil
	})

	log.Debugf("up2down")
	d.db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte("up2down"))

		b.ForEach(func(k, v []byte) error {
			log.Debugf("key=%d, value=%d\n", d.btoi(k), d.btoi(v))
			return nil
		})
		return nil
	})
}

// Path get DB path
func (d *Database) Path() string {
	return d.db.Path()
}

// Close close DB
func (d *Database) Close() {
	d.db.Close()
}

func (d *Database) itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func (d *Database) btoi(v []byte) int {
	return int(binary.BigEndian.Uint64(v))
}

// AddEvent add event to DB
func (d *Database) AddEvent(eventIDStr string) error {
	eventID, err := strconv.ParseInt(eventIDStr, 10, 64)
	if err != nil {
		panic(err)
	}

	eventIDBytes := d.itob(int(eventID))

	// Store the event id
	d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("events"))
		err := b.Put(eventIDBytes, d.itob(1))
		return err
	})

	return nil
}

// StoreMirror store mirror to DB
func (d *Database) StoreMirror(downstreamID int, upstreamID int) error {
	downstreamIDBytes := d.itob(downstreamID)
	upstreamIDBytes := d.itob(upstreamID)

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

// GetID get ID of item from DB
func (d *Database) GetID(bucket string, id int) (int, error) {
	// Start read-only transaction.
	tx, err := d.db.Begin(false)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	v := tx.Bucket([]byte(bucket)).Get(d.itob(id))
	if v == nil {
		//log.Debugf("Getting %d from %s = nil\n", id, bucket)
		return 0, nil
	}
	val := d.btoi(v)
	//log.Debugf("Getting %d from %s = %d\n", id, bucket, val)
	return val, nil
}

// SeenEvent have we seen this event before
func (d *Database) SeenEvent(eventIDStr string) (bool, error) {
	eventID, err := strconv.ParseInt(eventIDStr, 10, 64)
	if err != nil {
		panic("wat")
	}

	val, err := d.GetID("events", int(eventID))
	if val == 1 {
		return true, err
	}

	return false, err
}

// GetDownstreamID get id of downstream PR
func (d *Database) GetDownstreamID(upstreamID int) (int, error) {
	return d.GetID("up2down", upstreamID)
}

// GetUpstreamID get id of upstream PR
func (d *Database) GetUpstreamID(downstreamID int) (int, error) {
	return d.GetID("down2up", downstreamID)
}
