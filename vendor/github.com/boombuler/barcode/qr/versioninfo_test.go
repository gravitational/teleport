package qr

import "testing"

var testvi = &versionInfo{7, M, 0, 1, 10, 2, 5} // Fake versionInfo to run some of the tests

func Test_ErrorCorrectionStringer(t *testing.T) {
	tests := map[ErrorCorrectionLevel]string{
		L: "L", M: "M", Q: "Q", H: "H", ErrorCorrectionLevel(99): "unknown",
	}
	for ecl, str := range tests {
		if ecl.String() != str {
			t.Fail()
		}
	}
}

func Test_CharCountBits(t *testing.T) {
	v1 := &versionInfo{5, M, 0, 0, 0, 0, 0}
	v2 := &versionInfo{15, M, 0, 0, 0, 0, 0}
	v3 := &versionInfo{30, M, 0, 0, 0, 0, 0}

	if v1.charCountBits(numericMode) != 10 {
		t.Fail()
	}
	if v1.charCountBits(alphaNumericMode) != 9 {
		t.Fail()
	}
	if v1.charCountBits(byteMode) != 8 {
		t.Fail()
	}
	if v1.charCountBits(kanjiMode) != 8 {
		t.Fail()
	}
	if v2.charCountBits(numericMode) != 12 {
		t.Fail()
	}
	if v2.charCountBits(alphaNumericMode) != 11 {
		t.Fail()
	}
	if v2.charCountBits(byteMode) != 16 {
		t.Fail()
	}
	if v2.charCountBits(kanjiMode) != 10 {
		t.Fail()
	}
	if v3.charCountBits(numericMode) != 14 {
		t.Fail()
	}
	if v3.charCountBits(alphaNumericMode) != 13 {
		t.Fail()
	}
	if v3.charCountBits(byteMode) != 16 {
		t.Fail()
	}
	if v3.charCountBits(kanjiMode) != 12 {
		t.Fail()
	}
	if v1.charCountBits(encodingMode(3)) != 0 {
		t.Fail()
	}
}

func Test_TotalDataBytes(t *testing.T) {
	if testvi.totalDataBytes() != 20 {
		t.Fail()
	}
}

func Test_ModulWidth(t *testing.T) {
	if testvi.modulWidth() != 45 {
		t.Fail()
	}
}

func Test_FindSmallestVersionInfo(t *testing.T) {
	if findSmallestVersionInfo(H, alphaNumericMode, 10208) != nil {
		t.Error("there should be no version with this capacity")
	}
	test := func(cap int, tVersion byte) {
		v := findSmallestVersionInfo(H, alphaNumericMode, cap)
		if v == nil || v.Version != tVersion {
			t.Errorf("version %d should be returned.", tVersion)
		}
	}
	test(10191, 40)
	test(5591, 29)
	test(5592, 30)
	test(190, 3)
	test(200, 4)
}

type aligmnentTest struct {
	version  byte
	patterns []int
}

var allAligmnentTests = []*aligmnentTest{
	&aligmnentTest{1, []int{}},
	&aligmnentTest{2, []int{6, 18}},
	&aligmnentTest{3, []int{6, 22}},
	&aligmnentTest{4, []int{6, 26}},
	&aligmnentTest{5, []int{6, 30}},
	&aligmnentTest{6, []int{6, 34}},
	&aligmnentTest{7, []int{6, 22, 38}},
	&aligmnentTest{8, []int{6, 24, 42}},
	&aligmnentTest{9, []int{6, 26, 46}},
	&aligmnentTest{10, []int{6, 28, 50}},
	&aligmnentTest{11, []int{6, 30, 54}},
	&aligmnentTest{12, []int{6, 32, 58}},
	&aligmnentTest{13, []int{6, 34, 62}},
	&aligmnentTest{14, []int{6, 26, 46, 66}},
	&aligmnentTest{15, []int{6, 26, 48, 70}},
	&aligmnentTest{16, []int{6, 26, 50, 74}},
	&aligmnentTest{17, []int{6, 30, 54, 78}},
	&aligmnentTest{18, []int{6, 30, 56, 82}},
	&aligmnentTest{19, []int{6, 30, 58, 86}},
	&aligmnentTest{20, []int{6, 34, 62, 90}},
	&aligmnentTest{21, []int{6, 28, 50, 72, 94}},
	&aligmnentTest{22, []int{6, 26, 50, 74, 98}},
	&aligmnentTest{23, []int{6, 30, 54, 78, 102}},
	&aligmnentTest{24, []int{6, 28, 54, 80, 106}},
	&aligmnentTest{25, []int{6, 32, 58, 84, 110}},
	&aligmnentTest{26, []int{6, 30, 58, 86, 114}},
	&aligmnentTest{27, []int{6, 34, 62, 90, 118}},
	&aligmnentTest{28, []int{6, 26, 50, 74, 98, 122}},
	&aligmnentTest{29, []int{6, 30, 54, 78, 102, 126}},
	&aligmnentTest{30, []int{6, 26, 52, 78, 104, 130}},
	&aligmnentTest{31, []int{6, 30, 56, 82, 108, 134}},
	&aligmnentTest{32, []int{6, 34, 60, 86, 112, 138}},
	&aligmnentTest{33, []int{6, 30, 58, 86, 114, 142}},
	&aligmnentTest{34, []int{6, 34, 62, 90, 118, 146}},
	&aligmnentTest{35, []int{6, 30, 54, 78, 102, 126, 150}},
	&aligmnentTest{36, []int{6, 24, 50, 76, 102, 128, 154}},
	&aligmnentTest{37, []int{6, 28, 54, 80, 106, 132, 158}},
	&aligmnentTest{38, []int{6, 32, 58, 84, 110, 136, 162}},
	&aligmnentTest{39, []int{6, 26, 54, 82, 110, 138, 166}},
	&aligmnentTest{40, []int{6, 30, 58, 86, 114, 142, 170}},
}

func Test_AlignmentPatternPlacements(t *testing.T) {
	for _, at := range allAligmnentTests {
		vi := &versionInfo{at.version, M, 0, 0, 0, 0, 0}

		res := vi.alignmentPatternPlacements()
		if len(res) != len(at.patterns) {
			t.Errorf("number of alignmentpatterns missmatch for version %d", at.version)
		}
		for i := 0; i < len(res); i++ {
			if res[i] != at.patterns[i] {
				t.Errorf("alignmentpatterns for version %d missmatch on index %d", at.version, i)
			}
		}

	}

}
