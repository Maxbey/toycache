package resp

const (
	terminatorLength = 2
	terminator = "\r\n"
)

type dataType byte

const (
	String     dataType = '+'
	Error      dataType = '-'
	Integer    dataType = ':'
	BulkString dataType = '$'
	Array      dataType = '*'
)

type Element struct {
	Type     dataType
	Value    []byte
	Null     bool
	Elements []Element
}
