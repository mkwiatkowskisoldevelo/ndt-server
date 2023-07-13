package util

import (
	"crypto/tls"
	"fmt"
	"strings"
)

// ConvertCipherNamesToIdsArray takes a string containing the wanted cipher suites
// in string form, and transforms them into a slice of uint16 values. The
// cipher suite names are added to the string and are separated by comma.
// For instance, an input of:
//
//	"TLS_RSA_WITH_AES_128_CBC_SHA:TLS_RSA_WITH_AES_256_CBC_SHA"
//
// Would yield as output:
//
//	[]uint16{TLS_RSA_WITH_AES_128_CBC_SHA, TLS_RSA_WITH_AES_256_CBC_SHA}
func ConvertCipherNamesToIdsArray(ciphersString string) ([]uint16, error) {
	var result []uint16

	if ciphersString == "" {
		return nil, nil
	}
	ciphers := strings.Split(ciphersString, ",")
	for _, cipherName := range ciphers {
		cipherId := getCipherSuiteIdFromName(cipherName)
		if cipherId != 0 {
			result = append(result, cipherId)
		} else {
			return nil, fmt.Errorf("tls: failed to parse cipher suites string '%s'. Unkown cipher: '%s'", ciphersString, cipherName)
		}
	}
	if len(result) < 1 {
		return nil, fmt.Errorf("tls: cipher suites string '%s' didn't yield any results", ciphersString)
	}

	return result, nil
}

func getCipherSuiteIdFromName(name string) uint16 {
	for _, c := range tls.CipherSuites() {
		if c.Name == name {
			return c.ID
		}
	}
	for _, c := range tls.InsecureCipherSuites() {
		if c.Name == name {
			return c.ID
		}
	}
	return 0
}
