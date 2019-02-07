package runtime

const StringInternMaxLen = 64

func StringIntern(s string) string {
	if len(s) > StringInternMaxLen {
		return s
	}
	return stringintern(s)
}

// defined in runtime/string.go
func stringintern(string) string
