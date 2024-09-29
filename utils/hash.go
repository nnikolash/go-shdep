package utils

import (
	"crypto/md5"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/pkg/errors"
)

var hashJSONConfig = sonic.Config{SortMapKeys: true}.Froze()

func Hash(values ...interface{}) (string, error) {
	//return hashstructure.Hash(values, &hashstructure.HashOptions{})

	valuesStr, err := hashJSONConfig.MarshalIndent(values, "", "  ")
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal hashed values into json")
	}

	md5Hash := md5.New()
	if _, err := md5Hash.Write(valuesStr); err != nil {
		return "", errors.Wrapf(err, "failed to write hashed values into md5")
	}

	hash := fmt.Sprintf("%x", md5Hash.Sum(nil))

	return hash, nil
}
