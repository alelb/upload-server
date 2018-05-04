package message

// Message code
const (
	MissingHeaderTotalFileCount     = "missing_header_total_file_count"
	MissingHeaderCurrentFileCounter = "missing_header_current_file_counter"
	MissingHeaderCRC                = "missing_header_crc"
	ParseError                      = "parse_error"
	ChecksumFail                    = "checksum_fail"
	ErrorSlug                       = "error_slug"
	CountingError                   = "counting_error"
)

// Error is
type Error struct {
	Code string `json:"message_code"`
	Text string `json:"message_text"`
}

// NewError is
func NewError(code string, text string) Error {
	return Error{
		Code: code,
		Text: text,
	}
}
