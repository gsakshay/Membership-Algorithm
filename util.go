package main

import (
	"strconv"
	"strings"
)

func arrayToString(arr []int) string {
	var sb strings.Builder
	sb.WriteString("[")

	for i, val := range arr {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(strconv.Itoa(val))
	}

	sb.WriteString("]")
	return sb.String()
}
