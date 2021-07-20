package types

type StatsigEvent struct {
	EventName	string		 		`json:"eventName"`
	User		StatsigUser	 		`json:"user"`
	Value		string 				`json:"value"`
	Metadata	map[string]string	`json:"metadata"`
}