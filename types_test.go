package sri

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCodesAreTheSRIValues(t *testing.T) {
	assert.Equal(t, IDType("04"), IDRUC)
	assert.Equal(t, IDType("05"), IDCedula)
	assert.Equal(t, IDType("06"), IDPassport)
	assert.Equal(t, IDType("07"), IDFinalConsumer)
	assert.Equal(t, IDType("08"), IDForeign)
	assert.Equal(t, "9999999999999", FinalConsumerID)
	assert.Equal(t, "50", FinalConsumerLimit().String())
	assert.Equal(t, DocType("02"), DocSalesNote)
	assert.Equal(t, RimpeRegime(""), RimpeNone)
	assert.Equal(t, RimpeRegime("emprendedor"), RimpeEntrepreneur)
	assert.Equal(t, RimpeRegime("negocio_popular"), RimpePopularBusiness)
	assert.Equal(t, WithholdingTax("renta"), WithholdingRenta)
	assert.Equal(t, WithholdingTax("iva"), WithholdingIVA)
}
