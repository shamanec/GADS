package constants

type IndexSort int

const (
	SortAscending  IndexSort = 1
	SortDescending IndexSort = -1
)

var AndroidVersionToSDK = map[string]string{
	"23": "6",
	"24": "7",
	"25": "7",
	"26": "8",
	"27": "8",
	"28": "9",
	"29": "10",
	"30": "11",
	"31": "12",
	"32": "12",
	"33": "13",
	"34": "14",
}
