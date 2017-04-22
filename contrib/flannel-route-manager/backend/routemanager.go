package backend

type RouteManager interface {
	Delete(route string) (string, error)
	DeleteAllRoutes() ([]string, error)
	Insert(ip, subnet string) (string, error)
	Sync(map[string]string) (*SyncResponse, error)
}

type SyncResponse struct {
	Deleted  []string
	Inserted []string
}
