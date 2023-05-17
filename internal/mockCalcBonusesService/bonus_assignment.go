package mockCalcBonusesService

const (
	REGISTERED = "REGISTERED"
	INVALID    = "INVALID"
	PROCESSING = "PROCESSING"
	PROCESSED  = "PROCESSED"
)

type Bonus struct {
	OrderId int    `json:"order"`
	Status  string `json:"status"`
	Accrual int    `json:"accrual,omitempty"`
}
