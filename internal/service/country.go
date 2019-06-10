package service

import (
	"encoding/json"
	"fmt"
	"github.com/globalsign/mgo/bson"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
)

func newCountryService(svc *Service) *Country {
	s := &Country{svc: svc}
	return s
}

func (h *Country) Insert(country *billing.Country) error {
	if err := h.svc.db.Collection(pkg.CollectionCountry).Insert(country); err != nil {
		return err
	}

	key := fmt.Sprintf(pkg.CacheCountryCodeA2, country.CodeA2)
	if err := h.svc.cacher.Set(key, country, 0); err != nil {
		return err
	}

	return nil
}

func (h Country) MultipleInsert(country []*billing.Country) error {
	c := make([]interface{}, len(country))
	for i, v := range country {
		c[i] = v
	}

	if err := h.svc.db.Collection(pkg.CollectionCountry).Insert(c...); err != nil {
		return err
	}

	return nil
}

func (h Country) GetByCodeA2(code string) (*billing.Country, error) {
	c := &billing.Country{}
	key := fmt.Sprintf(pkg.CacheCountryCodeA2, code)
	res, err := h.svc.cacher.Get(key)

	if res != nil {
		err := json.Unmarshal(res, &c)
		if err != nil {
			return nil, fmt.Errorf(errorInterfaceCast, pkg.CollectionCountry)
		}
		return c, nil
	}

	err = h.svc.db.Collection(pkg.CollectionCountry).Find(bson.M{"is_active": true, "code_a2": code}).One(&c)
	if err != nil {
		return nil, fmt.Errorf(errorNotFound, pkg.CollectionCountry)
	}

	_ = h.svc.cacher.Set(key, c, 0)
	return c, nil
}
