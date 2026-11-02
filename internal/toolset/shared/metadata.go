package shared

// The API embeds the complete certificate model. Decode only the fields that
// may leave the panel, even when a transport has not applied its projection.
type SSLMetadata struct {
	ID            uint   `json:"id"`
	PrimaryDomain string `json:"primaryDomain"`
	Domains       string `json:"domains"`
	Type          string `json:"type"`
	Provider      string `json:"provider"`
	Organization  string `json:"organization"`
	AutoRenew     bool   `json:"autoRenew"`
	ExpireDate    string `json:"expireDate"`
	StartDate     string `json:"startDate"`
	Status        string `json:"status"`
	KeyType       string `json:"keyType"`
}
