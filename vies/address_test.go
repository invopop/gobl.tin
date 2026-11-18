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

// Company addresses from VIES (2026-11-10), plus made-up lines.
func TestParseAddressAT(t *testing.T) {
	at := func(street, num, code, locality string) *org.Address {
		return &org.Address{Street: street, Number: num, Code: cbc.Code(code), Locality: locality, Country: "AT"}
	}
	tests := []struct {
		name string
		raw  string
		want *org.Address
	}{
		{"street and number", "Innsbrucker Straße 75\nAT-6130 Schwaz", at("Innsbrucker Straße", "75", "6130", "Schwaz")},
		{"number with a letter", "Feldkirchenstraße 30b\nAT-8401 Kalsdorf b.Graz", at("Feldkirchenstraße", "30b", "8401", "Kalsdorf b.Graz")},
		{"letter after a space", "Siezenheimer Straße 29 A\nAT-5020 Salzburg", at("Siezenheimer Straße", "29 A", "5020", "Salzburg")},
		{"number range", "Achenseestr. 1-3\nAT-6200 Jenbach", at("Achenseestr.", "1-3", "6200", "Jenbach")},
		{"number range with spaces", "Wienerbergstraße 15 - 19\nAT-1100 Wien", at("Wienerbergstraße", "15 - 19", "1100", "Wien")},
		{"stair and door", "Argentinierstraße 18/1/10\nAT-1040 Wien", at("Argentinierstraße", "18/1/10", "1040", "Wien")},
		{"floor", "Strubergasse 28/5.OG\nAT-5020 Salzburg", at("Strubergasse", "28/5.OG", "5020", "Salzburg")},
		{"labelled stair and floor", "Neue Stiftingtalstraße 2/Stiege B6/OG\nAT-8010 Graz", at("Neue Stiftingtalstraße", "2/Stiege B6/OG", "8010", "Graz")},
		{"village without streets", "Anzing 33\nAT-4113 St.Martin/Mühlkreis", at("Anzing", "33", "4113", "St.Martin/Mühlkreis")},
		{"street with hyphens", "Dr.-Auner-Straße 19/5\nAT-8074 Raaba-Grambach", at("Dr.-Auner-Straße", "19/5", "8074", "Raaba-Grambach")},
		{"number in the street name", "IZ NÖ-Süd Str 3 Obj 16\nAT-2355 Wiener Neudorf", nil},
		{"no number", "EVN-Platz .\nAT-2344 Maria Enzersdorf", nil},
		{"building letter only", "Concorde Business Park F\nAT-2320 Schwechat", nil},
		{"comma after the number", "Wagramerstr 19,IZD Tower\nAT-1220 Wien", nil},
		{"PO box", "Postfach 123\nAT-1010 Wien", nil},
		{"market stall", "Schwendermarkt Stand/18\nAT-1150 Wien", nil},
		{"postcode without the country", "Innsbrucker Straße 75\n6130 Schwaz", nil},
		{"three lines", "c/o Muster GmbH\nInnsbrucker Straße 75\nAT-6130 Schwaz", nil},
		{"masked", "---", nil},
		{"empty", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseAddress("AT", tt.raw))
		})
	}
}

// Company addresses from VIES (2026-11-10), plus made-up lines.
func TestParseAddressPT(t *testing.T) {
	pt := func(street, num, extra, code, locality string) *org.Address {
		return &org.Address{Street: street, Number: num, StreetExtra: extra, Code: cbc.Code(code), Locality: locality, Country: "PT"}
	}
	tests := []struct {
		name string
		raw  string
		want *org.Address
	}{
		{"number marker", "RUA CASTILHO N 65 3 DTO\nLISBOA\n1250-068 LISBOA", pt("RUA CASTILHO", "65", "3 DTO", "1250-068", "LISBOA")},
		{"marker with commas", "RUA FORTE MONTE CINTRA, Nº 1, 2º C\nSACAVÉM\n2685-141 SACAVÉM", pt("RUA FORTE MONTE CINTRA", "1", "2º C", "2685-141", "SACAVÉM")},
		{"marker with a dot", "RUA 1º. DE MAIO Nº. 90\nFREIXO DE BAIXO AMT\n4600-614 FREIXO DE BAIXO", pt("RUA 1º. DE MAIO", "90", "", "4600-614", "FREIXO DE BAIXO")},
		{"marker in full", "RUA MARIANO CARVALHO NUMERO 230 5 A\nARCOZELO\n4410-483 ARCOZELO VNG", pt("RUA MARIANO CARVALHO", "230", "5 A", "4410-483", "ARCOZELO VNG")},
		{"letter after the number", "RUA LOMBO DA BOAVISTA N 10 A\nFUNCHAL\n9060-173 FUNCHAL", pt("RUA LOMBO DA BOAVISTA", "10 A", "", "9060-173", "FUNCHAL")},
		{"dash after the number", "RUA IRMÃOS SIEMENS N 1 - 1 A\nAMADORA\n2720-093 AMADORA", pt("RUA IRMÃOS SIEMENS", "1", "1 A", "2720-093", "AMADORA")},
		{"negative floor", "RUA CASTILHO N 65 -1 DTO\nLISBOA\n1250-068 LISBOA", pt("RUA CASTILHO", "65", "-1 DTO", "1250-068", "LISBOA")},
		{"number in the street name", "R 25 DE ABRIL N 55\nSILVARES\n4835-400 GUIMARAES", pt("R 25 DE ABRIL", "55", "SILVARES", "4835-400", "GUIMARAES")},
		{"road with a number", "ESTRADA NACIONAL 233 N 10 PANOIAS DE CIMA GUARDA\nPANOIAS DE CIMA\n6300-152 PANOIAS DE CIMA", pt("ESTRADA NACIONAL 233", "10", "PANOIAS DE CIMA GUARDA", "6300-152", "PANOIAS DE CIMA")},
		{"place other than the post town", "R DA BELA VISTA N 60\nGUALTAR\n4710-084 BRAGA", pt("R DA BELA VISTA", "60", "GUALTAR", "4710-084", "BRAGA")},
		{"letter and place", "RUA RIBEIRA JOÃO GOMES N 71 B\nSANTA LUZIA\n9050-563 FUNCHAL", pt("RUA RIBEIRA JOÃO GOMES", "71 B", "SANTA LUZIA", "9050-563", "FUNCHAL")},
		{"place without accents", "AV HEROIS DO ULTRAMAR N 56\nEVORA\n7005-161 ÉVORA", pt("AV HEROIS DO ULTRAMAR", "56", "", "7005-161", "ÉVORA")},
		{"place with hyphens", "ROT MARIO DIONISIO N 2\nLINDA-A-VELHA\n2799-557 LINDA A VELHA", pt("ROT MARIO DIONISIO", "2", "", "2799-557", "LINDA A VELHA")},
		{"place with brackets", "R DR JOSE BRUNO TAVARES CARREIRO N 6 9 PISO\nPONTA DELGADA (AÇORES)\n9500-769 PONTA DELGADA", pt("R DR JOSE BRUNO TAVARES CARREIRO", "6", "9 PISO", "9500-769", "PONTA DELGADA")},
		{"place with a leading space", "LARGO DR QUEIROS VAZ GUEDES N 3\n PONTE DA BARCA\n4980-618 PONTE DA BARCA", pt("LARGO DR QUEIROS VAZ GUEDES", "3", "", "4980-618", "PONTE DA BARCA")},
		{"number without a marker", "AVENIDA DA INDIA 8\nALCÂNTARA\n1349-062 LISBOA", pt("AVENIDA DA INDIA 8", "", "ALCÂNTARA", "1349-062", "LISBOA")},
		{"date in the street name", "AV 5 DE OUTUBRO\nFAFE\n4820-115 FAFE", pt("AV 5 DE OUTUBRO", "", "", "4820-115", "FAFE")},
		{"road number", "ESTRADA NACIONAL N 8 KM 52 AMIAL\nTORRES VEDRAS\n2565-641 RAMALHAL", pt("ESTRADA NACIONAL N 8 KM 52 AMIAL", "", "TORRES VEDRAS", "2565-641", "RAMALHAL")},
		{"lot number", "ZONA INDUSTRIAL DE VAGOS LOTE N 61\nVAGOS\n3840-385 VAGOS", pt("ZONA INDUSTRIAL DE VAGOS LOTE N 61", "", "", "3840-385", "VAGOS")},
		{"lot number with a dot", "ZONA INDUSTRIAL DE VAGOS LT. N 61\nVAGOS\n3840-385 VAGOS", pt("ZONA INDUSTRIAL DE VAGOS LT. N 61", "", "", "3840-385", "VAGOS")},
		{"road with a dot", "ESTRADA EN. N 8\nRAMALHAL\n2565-641 RAMALHAL", pt("ESTRADA EN. N 8", "", "", "2565-641", "RAMALHAL")},
		{"no street type", "CAMPUS DE CAMPOLIDE\nLISBOA\n1099-085 LISBOA", pt("CAMPUS DE CAMPOLIDE", "", "", "1099-085", "LISBOA")},
		{"no number", "RUA VALE DE CENTEIO S/N\nCERNACHE\n3040-807 CERNACHE", pt("RUA VALE DE CENTEIO S/N", "", "", "3040-807", "CERNACHE")},
		{"cut inside a word", "EDIFICIO ATLANTIS AV D JOÃO II N 44C PISO 5 PARQUE DAS NAÇOE\nS\nLISBOA\n1990-095 LISBOA", pt("EDIFICIO ATLANTIS AV D JOÃO II N 44C PISO 5 PARQUE DAS NAÇOES", "", "", "1990-095", "LISBOA")},
		{"cut before a space", "RUA JOAQUIM DIAS ROCHA 354 ZONA INDUSTRIAL DA MAIA 1 SECTOR \nX\nMAIA\n4470-211 MAIA", pt("RUA JOAQUIM DIAS ROCHA 354 ZONA INDUSTRIAL DA MAIA 1 SECTOR X", "", "", "4470-211", "MAIA")},
		{"cut after a space", "TALAIDE PARK CENTRO EMPRESARIAL TALAIDE ESTRADA OCTAVIO PATO\n N 175 177 EDIFICIO A 4\nSÃO DOMINGOS DE RANA\n2785-723 SAO DOMINGOS DE RANA", pt("TALAIDE PARK CENTRO EMPRESARIAL TALAIDE ESTRADA OCTAVIO PATO N 175 177 EDIFICIO A 4", "", "", "2785-723", "SAO DOMINGOS DE RANA")},
		{"cut and number marker", "ESTRADA COMANDANTE CAMACHO DE FREITAS N 529 PARKMADE ARMAZEM\n N 21 FRAÇÃO Q\nFUNCHAL\n9020-151 FUNCHAL", pt("ESTRADA COMANDANTE CAMACHO DE FREITAS", "529", "PARKMADE ARMAZEM N 21 FRAÇÃO Q", "9020-151", "FUNCHAL")},
		{"cut with a leading space", " GARE MARITIMA DA MADEIRA - MOLHE DA PONTINHA - PORTO DO FUN\nCHAL\nFUNCHAL\n9004-518 FUNCHAL", pt("GARE MARITIMA DA MADEIRA - MOLHE DA PONTINHA - PORTO DO FUNCHAL", "", "", "9004-518", "FUNCHAL")},
		{"PO box", "APARTADO 55\nMITRENA\n2910-738 SETUBAL", nil},
		{"PO box after a place", "VALE DE MOURO CAIXA POSTAL N 521 A\nFARO\n8005-538 FARO", nil},
		{"four lines without a cut", "RUA CASTILHO N 65\n3 DTO\nLISBOA\n1250-068 LISBOA", nil},
		{"two lines", "RUA CASTILHO N 65\n1250-068 LISBOA", nil},
		{"postcode without a hyphen", "RUA CASTILHO N 65\nLISBOA\n1250068 LISBOA", nil},
		{"masked", "---", nil},
		{"empty", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseAddress("PT", tt.raw))
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
