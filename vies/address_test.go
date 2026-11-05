package vies

import (
	"context"
	"net/http"
	"testing"

	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Company addresses from VIES (2026-10-15), plus made-up lines.
func TestParseAddressIT(t *testing.T) {
	it := func(street, num, extra, code, locality, region string) *org.Address {
		return &org.Address{Street: street, Number: num, StreetExtra: extra, Code: cbc.Code(code), Locality: locality, Region: region, Country: "IT"}
	}
	tests := []struct {
		name string
		raw  string
		want *org.Address
	}{
		{"street and number", "VIA EMILIA EST 1163 \n41122 MODENA MO\n", it("VIA EMILIA EST", "1163", "", "41122", "MODENA", "MO")},
		{"piazzale", "PIAZZALE ENRICO MATTEI 1 \n00144 ROMA RM\n", it("PIAZZALE ENRICO MATTEI", "1", "", "00144", "ROMA", "RM")},
		{"N before the number", "VIALE PIERO E ALBERTO PIRELLI N 25 \n20126 MILANO MI\n", it("VIALE PIERO E ALBERTO PIRELLI", "25", "", "20126", "MILANO", "MI")},
		{"number range", "VIA EUGENIO CHECCHI 17-19 \n00157 ROMA RM\n", it("VIA EUGENIO CHECCHI", "17-19", "", "00157", "ROMA", "RM")},
		{"number with a letter", "VIA DEI CERASETI 23A \n00047 MARINO RM\n", it("VIA DEI CERASETI", "23A", "", "00047", "MARINO", "RM")},
		{"flat after the number", "VIA DEL BOSCO DEGLI ARVALI 84 INT 7\n00148 ROMA RM\n", it("VIA DEL BOSCO DEGLI ARVALI", "84", "INT 7", "00148", "ROMA", "RM")},
		{"staircase and flat", "VIALE AMELIA 32 SC UN INT 14 \n00181 ROMA RM\n", it("VIALE AMELIA", "32", "SC UN INT 14", "00181", "ROMA", "RM")},
		{"N and flat", "VIA TRIONFALE N 85 INT 3 \n00136 ROMA RM\n", it("VIA TRIONFALE", "85", "INT 3", "00136", "ROMA", "RM")},
		{"number in the street name", "VIA 4 NOVEMBRE 10\n00187 ROMA RM", it("VIA 4 NOVEMBRE", "10", "", "00187", "ROMA", "RM")},
		{"no house number", "VIA 4 NOVEMBRE\n00187 ROMA RM", it("VIA 4 NOVEMBRE", "", "", "00187", "ROMA", "RM")},
		{"locality of several words", "VIA ROMA 1\n20090 SEGRATE MILANO MI", it("VIA ROMA", "1", "", "20090", "SEGRATE MILANO", "MI")},
		{"masked", "---", nil},
		{"empty", "", nil},
		{"one line", "VIA ROMA 1 00187 ROMA RM", nil},
		{"three lines", "C/O STUDIO ROSSI\nVIA ROMA 1\n00187 ROMA RM", nil},
		{"care-of line", "C/O STUDIO ROSSI\n00187 ROMA RM", nil},
		{"care-of line with a number", "C/O STUDIO ROSSI 12\n00187 ROMA RM", nil},
		{"street type with a dot", "LOC. SAN PIETRO 4\n06081 ASSISI PG", it("LOC. SAN PIETRO", "4", "", "06081", "ASSISI", "PG")},
		{"no province", "VIA ROMA 1\n00187 ROMA", nil},
		{"short postcode", "VIA ROMA 1\n0187 ROMA RM", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseAddress("IT", tt.raw))
		})
	}
}

// Company addresses from VIES (2026-10-22), plus made-up lines.
func TestParseAddressBG(t *testing.T) {
	bg := func(street, num, extra, code, locality, region string) *org.Address {
		return &org.Address{Street: street, Number: num, StreetExtra: extra, Code: cbc.Code(code), Locality: locality, Region: region, Country: "BG"}
	}
	tests := []struct {
		name string
		raw  string
		want *org.Address
	}{
		{"street and number", "ул. Гюешево  №14 обл.СОФИЯ, гр.СОФИЯ 1303", bg("ул. Гюешево", "14", "", "1303", "СОФИЯ", "СОФИЯ")},
		{"space after the number sign", "ул. ЙОРДАН ПЕТРОВ № 6 обл.СИЛИСТРА, гр.АЛФАТАР 7570", bg("ул. ЙОРДАН ПЕТРОВ", "6", "", "7570", "АЛФАТАР", "СИЛИСТРА")},
		{"No for the number sign", "кв. Камелия No 62 обл.БУРГАС, гр.НЕСЕБЪР 8230", bg("кв. Камелия", "62", "", "8230", "НЕСЕБЪР", "БУРГАС")},
		{"number with a letter", "бул.  \"Цариградско шосе\"  №115и обл.СОФИЯ, гр.СОФИЯ 1784", bg(`бул. "Цариградско шосе"`, "115и", "", "1784", "СОФИЯ", "СОФИЯ")},
		{"number with a separate letter", "ул. БЕЛАСИЦА  №20 А обл.ПЛЕВЕН, гр.ПЛЕВЕН 5800", bg("ул. БЕЛАСИЦА", "20 А", "", "5800", "ПЛЕВЕН", "ПЛЕВЕН")},
		{"number range", "ул. СТАРА ПЛАНИНА  №69-71 вх.Б ет.5 ап.27 обл.ХАСКОВО, гр.ХАСКОВО 6300", bg("ул. СТАРА ПЛАНИНА", "69-71", "вх.Б ет.5 ап.27", "6300", "ХАСКОВО", "ХАСКОВО")},
		{"bare number", "бул. ВИТОША 2 обл.СОФИЯ, гр.СОФИЯ 1065", bg("бул. ВИТОША", "2", "", "1065", "СОФИЯ", "СОФИЯ")},
		{"bare number with a letter", "бул. ПЕНЧО СЛАВЕЙКОВ 15-А обл.СОФИЯ, гр.СОФИЯ 1606", bg("бул. ПЕНЧО СЛАВЕЙКОВ", "15-А", "", "1606", "СОФИЯ", "СОФИЯ")},
		{"square", "пл. СВЕТА НЕДЕЛЯ   7 обл.СОФИЯ, гр.СОФИЯ 1000", bg("пл. СВЕТА НЕДЕЛЯ", "7", "", "1000", "СОФИЯ", "СОФИЯ")},
		{"no street type and a bare number", "Максим Горки 22 обл.ВРАЦА, гр.ВРАЦА 3000", bg("Максим Горки", "22", "", "3000", "ВРАЦА", "ВРАЦА")},
		{"no street type", "Максим Горки  №22 ет.4 ап.16 обл.ВРАЦА, гр.ВРАЦА 3000", bg("Максим Горки", "22", "ет.4 ап.16", "3000", "ВРАЦА", "ВРАЦА")},
		{"block without a number", "бул. ГЕНЕРАЛ КОЛЕВ бл.86 вх.А ет.1 ап.2 обл.ВАРНА, гр.ВАРНА 9002", bg("бул. ГЕНЕРАЛ КОЛЕВ", "", "бл.86 вх.А ет.1 ап.2", "9002", "ВАРНА", "ВАРНА")},
		{"space after the block label", "ул. ДРАГОЙНА бл. 12-14 ет.3 ап.7 обл.ХАСКОВО, гр.ХАСКОВО 6300", bg("ул. ДРАГОЙНА", "", "бл.12-14 ет.3 ап.7", "6300", "ХАСКОВО", "ХАСКОВО")},
		{"housing estate", " жк Изгрев  бл.189 ет.6 ап.30 обл.БУРГАС, гр.БУРГАС 8000", bg("жк Изгрев", "", "бл.189 ет.6 ап.30", "8000", "БУРГАС", "БУРГАС")},
		{"number in the estate name", " жк МЛАДОСТ 2 бл.210 вх.Б ет.4 ап.33 обл.СОФИЯ, гр.СОФИЯ 1799", bg("жк МЛАДОСТ 2", "", "бл.210 вх.Б ет.4 ап.33", "1799", "СОФИЯ", "СОФИЯ")},
		{"labels with commas", " жк ЛЕВ ТОЛСТОЙ, бл. 53, вх. В, ет. 3, ап. 55 обл.СОФИЯ, гр.СОФИЯ 1220", bg("жк ЛЕВ ТОЛСТОЙ", "", "бл.53 вх.В ет.3 ап.55", "1220", "СОФИЯ", "СОФИЯ")},
		{"office as the flat", "Симеоновско шосе  №93 Б вх.А ет.3 ап.офис 6 обл.СОФИЯ, гр.СОФИЯ 1700", bg("Симеоновско шосе", "93 Б", "вх.А ет.3 ап.офис 6", "1700", "СОФИЯ", "СОФИЯ")},
		{"office after a comma", "бул. \"Възкресение\"  №1, офис 7 обл.СОФИЯ, гр.СОФИЯ 1330", bg(`бул. "Възкресение"`, "1", "офис 7", "1330", "СОФИЯ", "СОФИЯ")},
		{"office with a number sign", "бул. \"8-ми Приморски Полк\"  №54 ет.4, офис № 22 обл.ВАРНА, гр.ВАРНА 9000", bg(`бул. "8-ми Приморски Полк"`, "54", "ет.4 офис 22", "9000", "ВАРНА", "ВАРНА")},
		{"ground floor", "\"6-ти Септември\"  №13 вх.А ет.Партерен обл.СОФИЯ, гр.СОФИЯ 1000", bg(`"6-ти Септември"`, "13", "вх.А ет.Партерен", "1000", "СОФИЯ", "СОФИЯ")},
		{"estate and street", " жк Гара Бяла ул.Никола Вапцаров  №51 обл.РУСЕ, гр.БЯЛА 7100", bg("ул.Никола Вапцаров", "51", "жк Гара Бяла", "7100", "БЯЛА", "РУСЕ")},
		{"estate glued to the street", " жк Дианабадул. Ловен парк  №1 вх.Б ет.3 ап.42 обл.СОФИЯ, гр.СОФИЯ 1172", bg("ул. Ловен парк", "1", "жк Дианабад вх.Б ет.3 ап.42", "1172", "СОФИЯ", "СОФИЯ")},
		{"estate, street and shop", " жк Малинова долина ул. \"Професор Живко Сталев\"  №24А, магазин 5 обл.СОФИЯ, гр.СОФИЯ 1734", bg(`ул. "Професор Живко Сталев"`, "24А", "жк Малинова долина магазин 5", "1734", "СОФИЯ", "СОФИЯ")},
		{"resort", " жк К.К.БОРОВЕЦ обл.СОФИЯ, гр.САМОКОВ 2000", bg("жк К.К.БОРОВЕЦ", "", "", "2000", "САМОКОВ", "СОФИЯ")},
		{"village", "ул. \"Шипка\"  №8А обл.ПЛЕВЕН, с.ВЪРБИЦА 5845", bg(`ул. "Шипка"`, "8А", "", "5845", "ВЪРБИЦА", "ПЛЕВЕН")},
		{"province of two words", "ул. \"Цар Освободител\"  №4 обл.ВЕЛИКО ТЪРНОВО, гр.ГОРНА ОРЯХОВИЦА 5100", bg(`ул. "Цар Освободител"`, "4", "", "5100", "ГОРНА ОРЯХОВИЦА", "ВЕЛИКО ТЪРНОВО")},
		{"tail without a comma", "ул.ВАСИЛ АПРИЛОВ бл.1 вх.А ет.4 ап.10 обл.ДОБРИЧ ГР.ДОБРИЧ 9300", bg("ул.ВАСИЛ АПРИЛОВ", "", "бл.1 вх.А ет.4 ап.10", "9300", "ДОБРИЧ", "ДОБРИЧ")},
		{"no postcode", "ул. Перперикон  №12 обл.КЪРДЖАЛИ, гр.КЪРДЖАЛИ", nil},
		{"two lines", "бул. ВИТОША 2\nобл.СОФИЯ, гр.СОФИЯ 1065", nil},
		{"full address and a second line", "ул. ВИТОША №2 обл.СОФИЯ, гр.СОФИЯ 1065\nет.3", nil},
		{"number at the end of the name", "ул. ОБОРИЩЕ 15 №3 обл.СОФИЯ, гр.СОФИЯ 1504", nil},
		{"text after a bare number", "бул. ВИТОША 2 В.ТЪРНОВО обл.СОФИЯ, гр.СОФИЯ 1000", nil},
		{"text after a bare number with a letter", "ул. ОБОРИЩЕ 15А ДВОР обл.СОФИЯ, гр.СОФИЯ 1504", nil},
		{"text after a bare number before a label", "ул. ОБОРИЩЕ 15 ДВОР №3 обл.СОФИЯ, гр.СОФИЯ 1504", nil},
		{"legacy columns", "УЛ.ВЕНЕЦ 028    обл.ГАБРОВО, гр.ГАБРОВО", nil},
		{"legacy columns with a flat", "УЛ.ВЕНЕЦ 028 01 04 007 обл.ГАБРОВО, гр.ГАБРОВО", nil},
		{"text after the number", "кв. Камелия No 62, хотел Заря обл.БУРГАС, гр.НЕСЕБЪР 8230", nil},
		{"name after the number", "Западна промишлена зона, ул. \"Перла\"  №2 \"Логистичен парк-Варна\" обл.ВАРНА, гр.ВАРНА 9009", nil},
		{"comma in the name", "Западна промишлена зона, ул. Перла обл.ВАРНА, гр.ВАРНА 9009", nil},
		{"glued words", " жк БАДЕМАкомплекс Селект  №616 обл.ХАСКОВО, гр.ХАСКОВО 6300", nil},
		{"letter after the number glued to a word", "ул. ВИТОША №5 В.ТЪРНОВО обл.СОФИЯ, гр.СОФИЯ 1000", nil},
		{"two numbers", "ул. ВИТОША №5 №7 обл.СОФИЯ, гр.СОФИЯ 1000", nil},
		{"no street", "обл.СОФИЯ, гр.СОФИЯ 1000", nil},
		{"no province", "ул. ВИТОША 2 гр.СОФИЯ 1000", nil},
		{"masked", "---", nil},
		{"empty", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseAddress("BG", tt.raw))
		})
	}
}

// Company addresses from VIES (2026-10-22), plus made-up lines.
func TestParseAddressNL(t *testing.T) {
	nl := func(street, num, extra, code, locality string) *org.Address {
		return &org.Address{Street: street, Number: num, StreetExtra: extra, Code: cbc.Code(code), Locality: locality, Country: "NL"}
	}
	tests := []struct {
		name string
		raw  string
		want *org.Address
	}{
		{"street and number", "\nDISTRIBUTIEWEG 00058\n2645EJ DELFGAUW\n", nl("DISTRIBUTIEWEG", "58", "", "2645EJ", "DELFGAUW")},
		{"street of several words", "\nBARUCH SPINOZASTRAAT 00010\n7442PD NIJVERDAL\n", nl("BARUCH SPINOZASTRAAT", "10", "", "7442PD", "NIJVERDAL")},
		{"number without padding", "\nKERKENBOS 01077 Q\n6546BB NIJMEGEN\n", nl("KERKENBOS", "1077 Q", "", "6546BB", "NIJMEGEN")},
		{"letter addition", "\nZEKERINGSTRAAT 00039 C\n1014BV AMSTERDAM\n", nl("ZEKERINGSTRAAT", "39 C", "", "1014BV", "AMSTERDAM")},
		{"addition with a hyphen", "\nMOLENSTRAAT 00017 A-1\n4881CP ZUNDERT\n", nl("MOLENSTRAAT", "17 A-1", "", "4881CP", "ZUNDERT")},
		{"number addition", "\nORANJESTRAAT 00008 14\n3111AP SCHIEDAM\n", nl("ORANJESTRAAT", "8 14", "", "3111AP", "SCHIEDAM")},
		{"unit", "\nSMEDERIJSTRAAT 00002 UNIT S. 2.07\n4814DB BREDA\n", nl("SMEDERIJSTRAAT", "2", "UNIT S. 2.07", "4814DB", "BREDA")},
		{"floor", "\nZEEBURGERDIJK 00490 3E VERD F\n1095AL AMSTERDAM\n", nl("ZEEBURGERDIJK", "490", "3E VERD F", "1095AL", "AMSTERDAM")},
		{"building name", "\nHOOGOORDDREEF 00060 CENTERP. 1\n1101BE AMSTERDAM\n", nl("HOOGOORDDREEF", "60", "CENTERP. 1", "1101BE", "AMSTERDAM")},
		{"place with an apostrophe", "\nLN COPES VAN CATTENBURCH 00046 C\n2585GB 'S-GRAVENHAGE\n", nl("LN COPES VAN CATTENBURCH", "46 C", "", "2585GB", "'S-GRAVENHAGE")},
		{"place with a province", "\nKORNEDIJK 00007 A\n4116CE BUREN GLD\n", nl("KORNEDIJK", "7 A", "", "4116CE", "BUREN GLD")},
		{"shielded address", "\nADRES AFGESCHERMD ART 00051\nLID 3 HANDELSREG.BESLUIT\n", nil},
		{"shielded address with a postcode", "ADRES AFGESCHERMD 00051\n1013HJ AMSTERDAM", nil},
		{"PO box", "POSTBUS 00123\n1013HJ AMSTERDAM", nil},
		{"care-of line", "WS AB T/O 260\nBROUWERSGRACHT 00001\n1013HJ AMSTERDAM\n", nil},
		{"number zero", "BROUWERSGRACHT 00000\n1013HJ AMSTERDAM", nil},
		{"foreign address", "SLINGERWEG 4\nB3600 GENK BELGIE\nBELGIE\n", nil},
		{"number not padded", "BROUWERSGRACHT 12\n1013HJ AMSTERDAM", nil},
		{"postcode with a space", "BROUWERSGRACHT 00012\n1013 HJ AMSTERDAM", nil},
		{"masked", "---", nil},
		{"empty", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseAddress("NL", tt.raw))
		})
	}
}

// Company addresses from VIES (2026-10-22), plus made-up lines.
func TestParseAddressPL(t *testing.T) {
	pl := func(street, num, extra, code, locality string) *org.Address {
		return &org.Address{Street: street, Number: num, StreetExtra: extra, Code: cbc.Code(code), Locality: locality, Country: "PL"}
	}
	tests := []struct {
		name string
		raw  string
		want *org.Address
	}{
		{"street and number", "BONIFRATERSKA 17\n00-203 WARSZAWA", pl("BONIFRATERSKA", "17", "", "00-203", "WARSZAWA")},
		{"number with a letter", "MATUSZEWSKA 14C\n03-876 WARSZAWA", pl("MATUSZEWSKA", "14C", "", "03-876", "WARSZAWA")},
		{"two numbers", "ŻEGAŃSKA 21/23\n04-713 WARSZAWA", pl("ŻEGAŃSKA", "21/23", "", "04-713", "WARSZAWA")},
		{"flat", "AL. ZWYCIĘSTWA 241/13 M4\n81-521 GDYNIA", pl("AL. ZWYCIĘSTWA", "241/13", "M4", "81-521", "GDYNIA")},
		{"flat with a dot", "DOMANIEWSKA 37 M2.43\n02-672 WARSZAWA", pl("DOMANIEWSKA", "37", "M2.43", "02-672", "WARSZAWA")},
		{"quotes in the street", "ALEJA \"SOLIDARNOŚCI\" 117 M207\n00-140 WARSZAWA", pl(`ALEJA "SOLIDARNOŚCI"`, "117", "M207", "00-140", "WARSZAWA")},
		{"post town of two words", "SANDOMIERSKA 26A M116\n27-400 OSTROWIEC ŚWIĘTOKRZYSKI", pl("SANDOMIERSKA", "26A", "M116", "27-400", "OSTROWIEC ŚWIĘTOKRZYSKI")},
		{"village with a post town", "NIEKANIN \nSZMARAGDOWA 3\n78-100 KOŁOBRZEG", pl("SZMARAGDOWA", "3", "NIEKANIN", "78-100", "KOŁOBRZEG")},
		{"village, flat and post town", "LATCHORZEW \nWARSZAWSKA 42/44 M21\n05-082 STARE BABICE", pl("WARSZAWSKA", "42/44", "M21, LATCHORZEW", "05-082", "STARE BABICE")},
		{"town without a post town", "SOSNOWIEC \nOSTROGÓRSKA 43\n41-200 ", pl("OSTROGÓRSKA", "43", "", "41-200", "SOSNOWIEC")},
		{"town equal to the post town", "GDAŃSK\nDŁUGA 75/76\n80-831 GDAŃSK", pl("DŁUGA", "75/76", "", "80-831", "GDAŃSK")},
		{"number at the start of the street", "3 MAJA 5\n00-001 WARSZAWA", pl("3 MAJA", "5", "", "00-001", "WARSZAWA")},
		{"number after an abbreviation", "AL. 3 MAJA 5\n00-001 WARSZAWA", pl("AL. 3 MAJA", "5", "", "00-001", "WARSZAWA")},
		{"no town", "OSTRÓWEK 82\n21-013 ", nil},
		{"unknown number", "SUCHA 1-3/2.L M018R\n50-186 WROCŁAW", nil},
		{"text after the number", "SUCHA 1 LOKAL 5\n50-186 WROCŁAW", nil},
		{"word starting with M after the number", "SUCHA 1 MAGAZYN\n50-186 WROCŁAW", nil},
		{"no number", "SUCHA\n50-186 WROCŁAW", nil},
		{"foreign address", "GETAFE \nMIGUEL FARADAY 1\n28906 \nHISZPANIA", nil},
		{"foreign postcode", "STATIONSTRAAT 40\n6515AB NIJMEGEN\nNIDERLANDY (HOLANDIA)", nil},
		{"town line with a number", "PARK 3\nSUCHA 1\n50-186 WROCŁAW", nil},
		{"masked", "---", nil},
		{"empty", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseAddress("PL", tt.raw))
		})
	}
}

func TestParseAddressWithoutParser(t *testing.T) {
	assert.Nil(t, parseAddress("DE", "MUSTERSTR. 1\n10115 BERLIN"), "a member state without a parser gives no address")
	assert.Nil(t, parseAddress("FR", "59 RUE LA FAYETTE\n75009 PARIS"), "France has no parser")
}

func TestVerifyAddress(t *testing.T) {
	t.Run("an Italian address is structured", func(t *testing.T) {
		v := serve(t, http.StatusOK, `{"countryCode":"IT","vatNumber":"00159560366","valid":true,"name":"FERRARI S.P.A.","address":"VIA EMILIA EST 1163 \n41122 MODENA MO\n"}`, nil)
		ans, err := v.Verify(context.Background(), tin.Request{Identifier: tin.Identifier{Path: tin.PathTaxID, Country: "IT", Code: "00159560366"}})
		require.NoError(t, err)
		require.NotNil(t, ans.Record)
		assert.Equal(t, "FERRARI S.P.A.", ans.Record.Name)
		assert.Equal(t, &org.Address{Street: "VIA EMILIA EST", Number: "1163", Code: "41122", Locality: "MODENA", Region: "MO", Country: "IT"}, ans.Record.Address)
	})

	t.Run("an address of an unknown shape is dropped and the name kept", func(t *testing.T) {
		v := serve(t, http.StatusOK, `{"countryCode":"IT","vatNumber":"00159560366","valid":true,"name":"FERRARI S.P.A.","address":"VIA EMILIA EST 1163 MODENA"}`, nil)
		ans, err := v.Verify(context.Background(), tin.Request{Identifier: tin.Identifier{Path: tin.PathTaxID, Country: "IT", Code: "00159560366"}})
		require.NoError(t, err)
		assert.Equal(t, &tin.Record{Name: "FERRARI S.P.A."}, ans.Record)
	})
}
