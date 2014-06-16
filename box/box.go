package box

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/binary"
	"io"
	"strconv"
)

type Ciphersuite interface {
	Name() [24]byte
	DHLen() int
	CCLen() int
	MACLen() int

	DH(privkey, pubkey []byte) []byte
	NewCipher(cv []byte) CipherContext
}

type CipherContext interface {
	Encrypt(dst, authtext, plaintext []byte) []byte
}

func deriveKey(secret, extraData, info []byte, outputLen int) []byte {
	output := make([]byte, 0, outputLen+sha512.Size)
	t := make([]byte, 0, sha512.Size)
	h := hmac.New(sha512.New, secret)

	// info || (byte)c || t[0:32] || extra_data
	data := make([]byte, len(info)+1+32+len(extraData))
	copy(data, info)
	copy(data[len(info)+1+32:], extraData)
	var c byte
	for len(output) < outputLen {
		data[len(info)] = c
		copy(data[len(info)+1:], t[:32])
		h.Write(data)
		t = h.Sum(t[:0])
		h.Reset()
		c++
		output = append(output, t...)
	}
	return output[:outputLen]
}

const cvLen = 48

type Key struct {
	Public  []byte
	Private []byte
}

func noiseBody(cc CipherContext, dst []byte, padLen int, appData, header []byte) []byte {
	plaintext := make([]byte, len(appData)+padLen+4)
	copy(plaintext, appData)
	if _, err := io.ReadFull(rand.Reader, plaintext[len(appData):len(appData)+padLen]); err != nil {
		panic(err)
	}
	binary.BigEndian.PutUint32(plaintext[len(appData)+padLen:], uint32(padLen))
	return cc.Encrypt(dst, header, plaintext)
}

func NoiseBox(c Ciphersuite, dst []byte, ephKey, senderKey Key, recvrPubkey []byte, padLen int, appData []byte, kdfNum int, cv []byte) ([]byte, []byte) {
	if len(cv) == 0 {
		cv = make([]byte, cvLen)
	}

	dh1 := c.DH(ephKey.Private, recvrPubkey)
	dh2 := c.DH(senderKey.Private, recvrPubkey)

	name := c.Name()
	cv1 := deriveKey(dh1, cv, strconv.AppendInt(name[:], int64(kdfNum), 10), cvLen+c.CCLen())
	cv2 := deriveKey(dh2, cv1, strconv.AppendInt(name[:], int64(kdfNum+1), 10), cvLen+c.CCLen())

	cc1 := c.NewCipher(cv1)
	cc2 := c.NewCipher(cv2)

	header := cc1.Encrypt(ephKey.Public, ephKey.Public, senderKey.Public)
	return noiseBody(cc2, header, padLen, appData, header), cv2
}
