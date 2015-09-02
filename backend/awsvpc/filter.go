package awsvpc

import "github.com/coreos/flannel/Godeps/_workspace/src/github.com/aws/aws-sdk-go/service/ec2"

type ecFilter []*ec2.Filter

func (f *ecFilter) Add(key, value string) {
	for _, fltr := range *f {
		if fltr.Name != nil && *fltr.Name == key {
			fltr.Values = append(fltr.Values, &value)
			return
		}
	}

	newFilter := &ec2.Filter{
		Name:   &key,
		Values: []*string{&value},
	}

	*f = append(*f, newFilter)
}

func newFilter() ecFilter {
	return make(ecFilter, 0)
}
