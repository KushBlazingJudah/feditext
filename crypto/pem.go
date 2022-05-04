package crypto

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

const keySize = 2048
const pemDir = "./pem"

const signWindow = 30

func writePem(name string, block *pem.Block) error {
	path := filepath.Join(pemDir, name+".pem")

	fp, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fp.Close()

	return pem.Encode(fp, block)
}

func getPrivateKey(id string) (*rsa.PrivateKey, error) {
	// TODO: Should this just be cached in memory?

	path := filepath.Join(pemDir, id+".private.pem")

	pkey, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(pkey)
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	return key, err
}

func PublicKey(id string) (string, error) {
	// TODO: Should this just be cached in memory?

	path := filepath.Join(pemDir, id+".pem")

	pkey, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(pkey), err
}

func CreatePem(id string) ([]byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return nil, err
	}

	pkey := x509.MarshalPKCS1PrivateKey(key)
	if err := writePem(id+".private", &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkey,
	}); err != nil {
		return nil, err
	}

	pubkey, err := x509.MarshalPKIXPublicKey(key.PublicKey)
	if err != nil {
		return pubkey, err
	}

	return pubkey, writePem(id, &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubkey,
	})
}

func Sign(id string, data string) (string, error) {
	key, err := getPrivateKey(id)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256([]byte(data))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])

	return base64.StdEncoding.EncodeToString(sig), err
}

func Verify(keyPem string, sign string, data string) error {
	sig, err := base64.StdEncoding.DecodeString(sign)
	if err != nil {
		return err
	}

	block, _ := pem.Decode([]byte(keyPem))
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return err
	}

	hash := sha256.Sum256([]byte(data))
	return rsa.VerifyPKCS1v15(key.(*rsa.PublicKey), crypto.SHA256, hash[:], sig)
}

func CheckHeaders(c *fiber.Ctx, keyPem string) error {
	// See https://blog.joinmastodon.org/2018/07/how-to-make-friends-and-verify-requests/

	// Check date for replay attacks
	t, err := time.Parse(time.RFC1123, c.GetReqHeaders()["Date"])
	if err != nil {
		return err
	}

	// Prevent reuse attacks
	if time.Now().UTC().Sub(t).Seconds() > signWindow {
		return fmt.Errorf("missed sign window")
	}

	out := []string{}
	sig := ""

	// Split up headers
	headers := ""
	for _, v := range strings.Split(c.GetReqHeaders()["Signature"], ",") {
		toks := strings.SplitN(v, "=", 2)
		if len(toks) != 2 {
			return fmt.Errorf("incorrectly formatted signature header")
		}

		k := toks[0]
		v = strings.TrimPrefix(strings.TrimSuffix(toks[1], "\""), "\"")
		switch strings.ToLower(k) {
		case "headers":
			headers = v
		case "signature":
			sig = v
		}
	}

	// Parse header
	for _, v := range strings.Split(headers, " ") {
		switch strings.ToLower(v) {
		case "(request-target)": // Ensure it's to _this_ path, not some other one.
			out = append(out, fmt.Sprintf("(request-target): %s %s", strings.ToLower(c.Method()), c.Path()))
		default:
			out = append(out, fmt.Sprintf("%s: %s", v, c.GetReqHeaders()[strings.Title(v)]))
		}
	}

	// TODO: Assumes digest is SHA256.
	return Verify(keyPem, sig, strings.Join(out, "\n"))
}
