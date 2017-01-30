package crm

import "github.com/denverdino/aliyungo/common"

const (
	FINANCE_SERIES = "aliyun.act_game"
	FINANCE_LABEL  = "act_finance_author" //金融云用户
)

type LabelSeriesArgs struct {
	LabelSeries string
}

type LabelSeries struct {
	Label       string
	LabelSeries string
}

type CustomerLabel struct {
	CustomerLabel []LabelSeries
}

type LabelSeriesResponse struct {
	common.Response

	Data CustomerLabel
}

func (client *Client) QueryCustomerLabel(labelSeries string) (*CustomerLabel, error) {
	args := LabelSeriesArgs{LabelSeries: labelSeries}
	response := LabelSeriesResponse{}
	err := client.Invoke("QueryCustomerLabel", &args, &response)
	if err != nil {
		return nil, err
	}

	return &response.Data, nil
}

func (client *Client) IsFinanceUser() bool {
	labels, err := client.QueryCustomerLabel(FINANCE_SERIES)
	if err == nil {
		for _, label := range labels.CustomerLabel {
			if label.Label == FINANCE_LABEL {
				return true
			}
		}
	}

	return false
}
