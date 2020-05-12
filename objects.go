package main

import (
	"strconv"
	"strings"
)

type ObjectContainer struct {
	Id      int64       `json:"id"`
	Type    string      `json:"type"`
	Method  string      `json:"method"`
	Content interface{} `json:"content"`
}

type ObjectIdentifiers map[int64]struct{}

func SerializeObjectIdentifiers(objects ObjectIdentifiers) (string, error) {
	var builder strings.Builder
	for id := range objects {
		_, err := builder.WriteString(strconv.FormatInt(id, 10))
		if err != nil {
			return builder.String(), err
		}
		err = builder.WriteByte(',')
		if err != nil {
			return builder.String(), err
		}
	}
	return builder.String(), nil
}

func DeserializeIdentifiers(serialized string) (ObjectIdentifiers, error) {
	result := make(map[int64]struct{})
	split := strings.Split(serialized, ",")
	for _, s := range split {
		if len(s) == 0 {
			continue
		}
		value, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return result, err
		}
		result[value] = struct{}{}
	}
	return result, nil
}
