package eigenda

type Config struct {
	// HTTP provider URL for the EigenDA disperser node.
	RPC string

	// The total amount of time that the sequencesender will spend waiting for EigenDA to confirm a blob
	StatusQueryTimeoutSeconds uint

	// The amount of time to wait between status queries of a newly dispersed blob
	StatusQueryRetryIntervalSeconds uint
}
