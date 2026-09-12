package service

import "github.com/WD-Mitchell/which-model/internal/company"

var readCompanyPolicy = company.Load

func requireCompanyCapability(capability string) error {
	policy, err := readCompanyPolicy()
	if err != nil {
		return err
	}
	return policy.RequireCapability(capability)
}

func requireCompanyProvider(provider string) error {
	policy, err := readCompanyPolicy()
	if err != nil {
		return err
	}
	return policy.RequireProvider(provider)
}
