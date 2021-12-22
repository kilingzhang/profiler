package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dgraph-io/badger/v3"
	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack/v5"
)

var (
	path     = "./data"
	badgerDB *badger.DB

	min []byte
	max []byte
)

func init() {
	var err error
	badgerDB, err = badger.Open(badger.DefaultOptions(path).WithLoggingLevel(3))
	if err != nil {
		panic(err)
	}
	min = BuildKey(time.Now().Add(-time.Hour * 24))
	max = BuildKey(time.Now())
}

func reset() {
	badgerDB.DropAll()
}

func BenchmarkInsertGob(b *testing.B) {
	reset()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		insert("gob", encodeGob())
	}
}

func BenchmarkInsertMsgPack(b *testing.B) {
	reset()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		insert("string", encodeMsgPack())
	}
}

func BenchmarkReadGob(b *testing.B) {
	reset()
	data := encodeGob()
	for i := 0; i < 100000; i++ {
		insert("gob", data)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		read(decodeGob)
	}
}

func BenchmarkReadMsgPack(b *testing.B) {
	reset()
	data := encodeMsgPack()
	for i := 0; i < 100000; i++ {
		insert("string", data)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		read(decodeMsgPack)
	}
}

func read(fn func(b []byte) *ProfileMeta) {
	target := &ProfileMetaByTarget{TargetName: "123", ProfileMetas: make([]*ProfileMeta, 0, 0)}
	err := badgerDB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			k := item.Key()
			if !CompareKey(k, max) {
				break
			}
			err := item.Value(func(v []byte) error {
				target.ProfileMetas = append(target.ProfileMetas, fn(v))
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}

func insert(prefix string, data []byte) {
	seq, err := badgerDB.GetSequence([]byte(prefix), 1000000)
	if err != nil {
		panic(err)
	}
	err = badgerDB.Update(func(txn *badger.Txn) error {
		num, err := seq.Next()
		if err != nil {
			return err
		}
		idb := itob(int(num))
		key := append([]byte(prefix), BuildKey(time.Now())...)
		key = append(key, idb...)
		return txn.Set(key, data)
	})
	if err != nil {
		panic(err)
	}
}

func itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func encodeGob() []byte {
	sample := &ProfileMeta{}
	sample.ProfileID = 1
	sample.Timestamp = time.Now().UnixNano() / time.Millisecond.Nanoseconds()
	sample.Duration = time.Now().UnixNano()
	sample.SampleTypeUnit = "count"
	sample.SampleType = "alloc_objects"
	var sampleBuf bytes.Buffer
	err := gob.NewEncoder(&sampleBuf).Encode(sample)
	if err != nil {
		panic(err)
	}
	return sampleBuf.Bytes()
}

func encodeMsgPack() []byte {
	sample := &ProfileMeta{}
	sample.ProfileID = 1
	sample.Timestamp = time.Now().UnixNano() / time.Millisecond.Nanoseconds()
	sample.Duration = time.Now().UnixNano()
	sample.SampleTypeUnit = "count"
	sample.SampleType = "alloc_objects"
	b, err := msgpack.Marshal(&sample)
	if err != nil {
		panic(err)
	}
	return b
}

func decodeGob(v []byte) *ProfileMeta {
	var sample ProfileMeta
	buf := bytes.NewBuffer(v)
	err := gob.NewDecoder(buf).Decode(&sample)
	if err != nil {
	}
	return &sample
}

func decodeMsgPack(v []byte) *ProfileMeta {
	var sample ProfileMeta
	err := msgpack.Unmarshal(v, &sample)
	if err != nil {
		panic(err)
	}
	return &sample
}

func TestEncode(t *testing.T) {
	sample := &ProfileMeta{}
	sample.ProfileID = 1
	sample.Timestamp = time.Now().UnixNano() / time.Millisecond.Nanoseconds()
	sample.Duration = time.Now().UnixNano()
	sample.SampleTypeUnit = "count"
	sample.SampleType = "alloc_objects"
	b, err := sample.Encode()
	require.Equal(t, err, nil)

	var sample1 ProfileMeta
	err = sample1.Decode(b)
	require.Equal(t, err, nil)
	require.Equal(t, sample.ProfileID, sample1.ProfileID)
	require.Equal(t, sample.Timestamp, sample1.Timestamp)
	require.Equal(t, sample.Duration, sample1.Duration)
	require.Equal(t, sample.SampleTypeUnit, sample1.SampleTypeUnit)
	require.Equal(t, sample.SampleType, sample1.SampleType)
}
