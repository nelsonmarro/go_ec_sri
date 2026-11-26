package sri

import (
	"net/mail"
	"regexp"
	"strings"
)

var (
	tenDigits      = regexp.MustCompile(`^\d{10}$`)
	thirteenDigits = regexp.MustCompile(`^\d{13}$`)
	foreignID      = regexp.MustCompile(`^[A-Za-z0-9-]{3,13}$`)
)

// validProvince reports whether the first two digits are a province code:
// 01 to 24, or 30 (Ecuadorians abroad).
func validProvince(id string) bool {
	p := int(id[0]-'0')*10 + int(id[1]-'0')
	return (p >= 1 && p <= 24) || p == 30
}

// ValidRUC reports whether s has the structure of a RUC: 13 digits, a valid
// province and the 001 suffix. The módulo 11 check digit is not validated on
// purpose: the SRI has issued RUCs that do not satisfy it.
func ValidRUC(s string) bool {
	return thirteenDigits.MatchString(s) && validProvince(s) && s[10:] == "001"
}

// ValidCedula reports whether s is a valid cédula: 10 digits, a valid
// province and the módulo 10 check digit.
func ValidCedula(s string) bool {
	if !tenDigits.MatchString(s) || !validProvince(s) {
		return false
	}
	sum := 0
	for i := range 9 {
		d := int(s[i] - '0')
		if i%2 == 0 {
			if d *= 2; d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	return (10-sum%10)%10 == int(s[9]-'0')
}

// validForeignID reports whether s is a passport or foreign identification:
// 3 to 13 letters, digits or hyphens.
func validForeignID(s string) bool { return foreignID.MatchString(s) }

// validEmail reports whether s is one plain address (no display name, no
// list) with a dotted domain.
func validEmail(s string) bool {
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s {
		return false
	}
	_, domain, _ := strings.Cut(s, "@")
	return strings.Contains(domain, ".")
}
