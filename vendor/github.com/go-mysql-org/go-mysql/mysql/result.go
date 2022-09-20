package mysql

type Result struct {
	Status   uint16
	Warnings uint16

	InsertId     uint64
	AffectedRows uint64

	*Resultset
}

type Executer interface {
	Execute(query string, args ...interface{}) (*Result, error)
}

func (r *Result) Close() {
	if r.Resultset != nil {
		r.Resultset.returnToPool()
		r.Resultset = nil
	}
}
