// Code generated by "stringer -type AtomType"; DO NOT EDIT.

package terex

import "strconv"

const _AtomType_name = "NoTypeConsTypeVarTypeNumTypeStringTypeBoolTypeOperatorTypeTokenTypeEnvironmentTypeUserTypeAnyTypeErrorType"

var _AtomType_index = [...]uint8{0, 6, 14, 21, 28, 38, 46, 58, 67, 82, 90, 97, 106}

func (i AtomType) String() string {
	if i < 0 || i >= AtomType(len(_AtomType_index)-1) {
		return "AtomType(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _AtomType_name[_AtomType_index[i]:_AtomType_index[i+1]]
}