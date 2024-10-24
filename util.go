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

func nextLeader(n int, arr []int) (int, bool) {
	if len(arr) == 0 {
		return 0, false
	}

	for _, value := range arr {
		if value > n {
			return value, true
		}
	}

	return 0, false
}
