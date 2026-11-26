package sri

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"strconv"
	"time"
)

var (
	threeDigits       = regexp.MustCompile(`^\d{3}$`)
	nineDigits        = regexp.MustCompile(`^\d{9}$`)
	accessKeyPattern  = regexp.MustCompile(`^\d{49}$`)
	numericCodeBound  = big.NewInt(100_000_000)
	emissionTypeCode  = "1" // normal emission (offline scheme): the only one the SRI allows today
	accessKeyDateForm = "02012006"
)

// GenerateAccessKey builds the 49-digit access key: date (ddmmaaaa), document
// type, RUC, environment, establishment, emission point, sequential, an
// 8-digit numeric code drawn from random, emission type 1 and the módulo 11
// check digit. The RUC must pass ValidRUC. Callers pass crypto/rand.Reader in
// production and a fixed reader in tests.
func GenerateAccessKey(issueDate time.Time, doc DocType, ruc string, env Environment, establishment, emissionPoint, sequential string, random io.Reader) (string, error) {
	if !ValidRUC(ruc) {
		return "", errors.New("el RUC debe tener 13 dígitos, una provincia válida y terminar en 001")
	}
	if env != EnvironmentTest && env != EnvironmentProduction {
		return "", errors.New("el ambiente del SRI debe ser pruebas (1) o producción (2)")
	}
	if doc != DocInvoice && doc != DocCreditNote && doc != DocWithholding {
		return "", errors.New("el tipo de comprobante debe ser 01, 04 o 07")
	}
	if !threeDigits.MatchString(establishment) || !threeDigits.MatchString(emissionPoint) || !nineDigits.MatchString(sequential) {
		return "", errors.New("establecimiento, punto de emisión y secuencial deben tener 3, 3 y 9 dígitos")
	}
	n, err := rand.Int(random, numericCodeBound)
	if err != nil {
		return "", fmt.Errorf("generar el código numérico de la clave de acceso: %w", err)
	}
	key48 := issueDate.Format(accessKeyDateForm) + string(doc) + ruc + strconv.Itoa(int(env)) +
		establishment + emissionPoint + sequential + fmt.Sprintf("%08d", n.Int64()) + emissionTypeCode
	return key48 + strconv.Itoa(checkDigit(key48)), nil
}

// ValidAccessKey reports whether key has 49 digits and a correct check digit.
func ValidAccessKey(key string) bool {
	if !accessKeyPattern.MatchString(key) {
		return false
	}
	return strconv.Itoa(checkDigit(key[:48])) == key[48:]
}

// AccessKeyParts are the fields encoded in an access key.
type AccessKeyParts struct {
	IssueDate                                time.Time
	Doc                                      DocType
	RUC                                      string
	Environment                              Environment
	Establishment, EmissionPoint, Sequential string
	NumericCode, EmissionType                string
}

// ParseAccessKey splits a valid access key into its fields.
func ParseAccessKey(key string) (AccessKeyParts, error) {
	if !ValidAccessKey(key) {
		return AccessKeyParts{}, errors.New("la clave de acceso no es válida (49 dígitos con su dígito verificador)")
	}
	date, err := time.Parse(accessKeyDateForm, key[0:8])
	if err != nil {
		return AccessKeyParts{}, fmt.Errorf("la fecha de la clave de acceso no es válida: %w", err)
	}
	env, _ := strconv.Atoi(key[23:24]) // digits are guaranteed by ValidAccessKey
	return AccessKeyParts{
		IssueDate:     date,
		Doc:           DocType(key[8:10]),
		RUC:           key[10:23],
		Environment:   Environment(env),
		Establishment: key[24:27],
		EmissionPoint: key[27:30],
		Sequential:    key[30:39],
		NumericCode:   key[39:47],
		EmissionType:  key[47:48],
	}, nil
}

// checkDigit computes the módulo 11 check digit with weights 2..7 from the
// right; 11 maps to 0 and 10 maps to 1.
func checkDigit(digits string) int {
	sum, factor := 0, 2
	for i := len(digits) - 1; i >= 0; i-- {
		sum += int(digits[i]-'0') * factor
		factor++
		if factor > 7 {
			factor = 2
		}
	}
	switch res := 11 - sum%11; res {
	case 11:
		return 0
	case 10:
		return 1
	default:
		return res
	}
}
