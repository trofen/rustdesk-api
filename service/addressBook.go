package service

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"gorm.io/gorm"
	"strings"
)

type AddressBookService struct {
}

func (s *AddressBookService) Info(id string) *model.AddressBook {
	p := &model.AddressBook{}
	DB.Where("id = ?", id).First(p)
	return p
}

func (s *AddressBookService) InfoByUserIdAndId(userid uint, id string) *model.AddressBook {
	p := &model.AddressBook{}
	DB.Where("user_id = ? and id = ?", userid, id).First(p)
	return p
}

func (s *AddressBookService) InfoByUserIdAndIdAndCid(userid uint, id string, cid uint) *model.AddressBook {
	p := &model.AddressBook{}
	DB.Where("user_id = ? and id = ? and collection_id = ?", userid, id, cid).First(p)
	return p
}
func (s *AddressBookService) InfoByRowId(id uint) *model.AddressBook {
	p := &model.AddressBook{}
	DB.Where("row_id = ?", id).First(p)
	return p
}
func (s *AddressBookService) ListByUserId(userId, page, pageSize uint) (res *model.AddressBookList) {
	res = s.List(page, pageSize, func(tx *gorm.DB) {
		tx.Where("user_id = ?", userId)
	})
	return
}
func (s *AddressBookService) ListByUserIds(userIds []uint, page, pageSize uint) (res *model.AddressBookList) {
	res = s.List(page, pageSize, func(tx *gorm.DB) {
		tx.Where("user_id in (?)", userIds)
	})
	return
}

// AddAddressBook
func (s *AddressBookService) AddAddressBook(ab *model.AddressBook) error {
	return DB.Create(ab).Error
}

// UpdateAddressBook
func (s *AddressBookService) UpdateAddressBook(abs []*model.AddressBook, userId uint) error {
	//比较peers和数据库中的数据，如果peers中的数据在数据库中不存在，则添加，如果存在则更新，如果数据库中的数据在peers中不存在，则删除
	// 开始事务
	tx := DB.Begin()
	//1. 获取数据库中的数据
	var dbABs []*model.AddressBook
	tx.Where("user_id = ?", userId).Find(&dbABs)
	//2. 比较peers和数据库中的数据
	//2.1 获取peers中的id
	aBIds := make(map[string]*model.AddressBook)
	for _, ab := range abs {
		aBIds[ab.Id] = ab
	}
	//2.2 获取数据库中的id
	dbABIds := make(map[string]*model.AddressBook)
	for _, dbAb := range dbABs {
		dbABIds[dbAb.Id] = dbAb
	}
	//2.3 比较peers和数据库中的数据
	for id, ab := range aBIds {
		dbAB, ok := dbABIds[id]
		ab.UserId = userId
		if !ok {
			//添加
			if ab.Platform == "" || ab.Username == "" || ab.Hostname == "" {
				peer := AllService.PeerService.FindById(ab.Id)
				if peer.RowId != 0 {
					ab.Platform = AllService.AddressBookService.PlatformFromOs(peer.Os)
					ab.Username = peer.Username
					ab.Hostname = peer.Hostname
				}
			}
			tx.Create(ab)
		} else {
			//更新
			tx.Model(&model.AddressBook{}).Where("row_id = ?", dbAB.RowId).Updates(ab)
		}
	}
	//2.4 删除
	for id, dbAB := range dbABIds {
		_, ok := aBIds[id]
		if !ok {
			tx.Delete(dbAB)
		}
	}
	tx.Commit()
	return nil

}

func (s *AddressBookService) List(page, pageSize uint, where func(tx *gorm.DB)) (res *model.AddressBookList) {
	res = &model.AddressBookList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.AddressBook{})
	if where != nil {
		where(tx)
	}
	tx.Count(&res.Total)
	tx.Scopes(Paginate(page, pageSize))
	tx.Find(&res.AddressBooks)
	return
}

func (s *AddressBookService) FromPeer(peer *model.Peer) (a *model.AddressBook) {
	a = &model.AddressBook{}
	a.Id = peer.Id
	a.Username = peer.Username
	a.Hostname = peer.Hostname
	a.UserId = peer.UserId
	a.Platform = s.PlatformFromOs(peer.Os)
	return a
}

// Create adds an address book record.
func (s *AddressBookService) Create(u *model.AddressBook) error {
	res := DB.Create(u).Error
	return res
}
func (s *AddressBookService) Delete(u *model.AddressBook) error {
	tx := DB.Begin()
	if err := tx.Where("address_book_row_id = ?", u.RowId).Delete(&model.AddressBookRule{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Delete(u).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

// Update changes an address book record.
func (s *AddressBookService) Update(u *model.AddressBook) error {
	return DB.Model(u).Updates(u).Error
}

// UpdateByMap changes selected address book fields.
func (s *AddressBookService) UpdateByMap(u *model.AddressBook, data map[string]interface{}) error {
	return DB.Model(u).Updates(data).Error
}

// UpdateAll replaces all editable address book fields and keeps record rules in the same collection.
func (s *AddressBookService) UpdateAll(u *model.AddressBook) error {
	existing := s.InfoByRowId(u.RowId)
	tx := DB.Begin()
	if err := tx.Model(u).Select("*").Omit("created_at").Updates(u).Error; err != nil {
		tx.Rollback()
		return err
	}
	if existing.RowId != 0 && (existing.CollectionId != u.CollectionId || existing.UserId != u.UserId) {
		if err := tx.Model(&model.AddressBookRule{}).
			Where("address_book_row_id = ?", u.RowId).
			Updates(map[string]interface{}{
				"collection_id": u.CollectionId,
				"user_id":       u.UserId,
			}).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

// ShareByWebClient creates a web-client share token.
func (s *AddressBookService) ShareByWebClient(m *model.ShareRecord) error {
	m.ShareToken = uuid.New().String()
	return DB.Create(m).Error
}

// SharedPeer
func (s *AddressBookService) SharedPeer(shareToken string) *model.ShareRecord {
	m := &model.ShareRecord{}
	DB.Where("share_token = ?", shareToken).First(m)
	return m
}

// PlatformFromOs
func (s *AddressBookService) PlatformFromOs(os string) string {
	if strings.Contains(os, "Android") || strings.Contains(os, "android") {
		return "Android"
	}
	if strings.Contains(os, "Windows") || strings.Contains(os, "windows") {
		return "Windows"
	}
	if strings.Contains(os, "Linux") || strings.Contains(os, "linux") {
		return "Linux"
	}
	if strings.Contains(os, "mac") || strings.Contains(os, "Mac") {
		return "Mac OS"
	}
	return ""
}
func (s *AddressBookService) ListByUserIdAndCollectionId(userId, cid, page, pageSize uint) (res *model.AddressBookList) {
	res = s.List(page, pageSize, func(tx *gorm.DB) {
		tx.Where("user_id = ? and collection_id = ?", userId, cid)
	})
	return
}
func (s *AddressBookService) ListCollection(page, pageSize uint, where func(tx *gorm.DB)) (res *model.AddressBookCollectionList) {
	res = &model.AddressBookCollectionList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.AddressBookCollection{})
	if where != nil {
		where(tx)
	}
	tx.Count(&res.Total)
	tx.Scopes(Paginate(page, pageSize))
	tx.Find(&res.AddressBookCollection)
	return
}
func (s *AddressBookService) ListCollectionByIds(ids []uint) (res []*model.AddressBookCollection) {
	DB.Where("id in ?", ids).Find(&res)
	return res
}

func (s *AddressBookService) ListCollectionByUserId(userId uint) (res *model.AddressBookCollectionList) {
	res = s.ListCollection(1, 100, func(tx *gorm.DB) {
		tx.Where("user_id = ?", userId)
	})
	return
}
func (s *AddressBookService) CollectionInfoById(id uint) *model.AddressBookCollection {
	p := &model.AddressBookCollection{}
	DB.Where("id = ?", id).First(p)
	return p
}

func (s *AddressBookService) CollectionReadRules(user *model.User) (res []*model.AddressBookCollectionRule) {
	// Personal rules that grant read access.
	var personalRules []*model.AddressBookCollectionRule
	tx2 := DB.Model(&model.AddressBookCollectionRule{})
	tx2.Where("type = ? and to_id = ? and rule > 0", model.ShareAddressBookRuleTypePersonal, user.Id).Find(&personalRules)
	res = append(res, personalRules...)

	if user.GroupId > 0 {
		// Group rules that grant read access.
		var groupRules []*model.AddressBookCollectionRule
		tx3 := DB.Model(&model.AddressBookCollectionRule{})
		tx3.Where("type = ? and to_id = ? and rule > 0", model.ShareAddressBookRuleTypeGroup, user.GroupId).Find(&groupRules)
		res = append(res, groupRules...)
	}
	return
}

func ResolveRuleByPriority(inherited int, personalRule, groupRule *int) int {
	if personalRule != nil {
		return *personalRule
	}
	if groupRule != nil {
		return *groupRule
	}
	return inherited
}

func ResolveOwnedRule(viewerID, ownerID uint, inherited int, personalRule, groupRule *int) int {
	if viewerID == ownerID {
		return model.ShareAddressBookRuleRuleFullControl
	}
	return ResolveRuleByPriority(inherited, personalRule, groupRule)
}

func ResolveAddressBookRule(inherited int, rowID uint, personalRules, groupRules map[uint]int) int {
	if rule, ok := personalRules[rowID]; ok {
		return rule
	}
	if rule, ok := groupRules[rowID]; ok {
		return rule
	}
	return inherited
}

func splitAddressBookRuleMaps(rules []*model.AddressBookRule) (personalRules, groupRules map[uint]int) {
	personalRules = make(map[uint]int)
	groupRules = make(map[uint]int)
	for _, rule := range rules {
		if rule.Type == model.ShareAddressBookRuleTypePersonal {
			personalRules[rule.AddressBookRowId] = rule.Rule
		}
		if rule.Type == model.ShareAddressBookRuleTypeGroup {
			groupRules[rule.AddressBookRowId] = rule.Rule
		}
	}
	return personalRules, groupRules
}

func (s *AddressBookService) UserMaxRule(user *model.User, uid, cid uint) int {
	if user == nil || user.Id == 0 {
		return model.ShareAddressBookRuleRuleNone
	}

	personalRules := &model.AddressBookCollectionRule{}
	tx := DB.Model(personalRules)
	tx.Where("type = ? and collection_id = ? and to_id = ?", model.ShareAddressBookRuleTypePersonal, cid, user.Id).First(&personalRules)
	var personalRule *int
	if personalRules.Id != 0 {
		personalRule = &personalRules.Rule
	}

	groupRules := &model.AddressBookCollectionRule{}
	var groupRule *int
	if user.GroupId > 0 {
		tx2 := DB.Model(groupRules)
		tx2.Where("type = ? and collection_id = ? and to_id = ?", model.ShareAddressBookRuleTypeGroup, cid, user.GroupId).First(&groupRules)
		if groupRules.Id != 0 {
			groupRule = &groupRules.Rule
		}
	}
	return ResolveOwnedRule(user.Id, uid, model.ShareAddressBookRuleRuleNone, personalRule, groupRule)
}

func (s *AddressBookService) UserAddressBookRule(user *model.User, uid, cid, rowID uint) int {
	if user == nil || user.Id == 0 {
		return model.ShareAddressBookRuleRuleNone
	}
	inherited := s.UserMaxRule(user, uid, cid)
	if user.Id == uid {
		return inherited
	}

	personalRule := &model.AddressBookRule{}
	DB.Where("type = ? and address_book_row_id = ? and to_id = ?", model.ShareAddressBookRuleTypePersonal, rowID, user.Id).First(personalRule)
	var personalRuleValue *int
	if personalRule.Id != 0 {
		personalRuleValue = &personalRule.Rule
	}

	groupRule := &model.AddressBookRule{}
	var groupRuleValue *int
	if user.GroupId > 0 {
		DB.Where("type = ? and address_book_row_id = ? and to_id = ?", model.ShareAddressBookRuleTypeGroup, rowID, user.GroupId).First(groupRule)
		if groupRule.Id != 0 {
			groupRuleValue = &groupRule.Rule
		}
	}
	return ResolveOwnedRule(user.Id, uid, inherited, personalRuleValue, groupRuleValue)
}

func (s *AddressBookService) AddressBookRuleMapsForUserAndCollection(user *model.User, cid uint) (map[uint]int, map[uint]int) {
	rules := s.AddressBookRulesForUserAndCollection(user, cid)
	return splitAddressBookRuleMaps(rules)
}

func (s *AddressBookService) CheckUserReadPrivilege(user *model.User, uid, cid uint) bool {
	return s.UserMaxRule(user, uid, cid) >= model.ShareAddressBookRuleRuleRead
}
func (s *AddressBookService) CheckUserWritePrivilege(user *model.User, uid, cid uint) bool {
	return s.UserMaxRule(user, uid, cid) >= model.ShareAddressBookRuleRuleReadWrite
}
func (s *AddressBookService) CheckUserFullControlPrivilege(user *model.User, uid, cid uint) bool {
	return s.UserMaxRule(user, uid, cid) >= model.ShareAddressBookRuleRuleFullControl
}

func (s *AddressBookService) CreateCollection(t *model.AddressBookCollection) error {
	return DB.Create(t).Error
}

func (s *AddressBookService) UpdateCollection(t *model.AddressBookCollection) error {
	return DB.Model(t).Updates(t).Error
}

func (s *AddressBookService) DeleteCollection(t *model.AddressBookCollection) error {
	// Delete all rules and address book records before deleting the collection.
	tx := DB.Begin()
	tx.Where("collection_id = ?", t.Id).Delete(&model.AddressBookCollectionRule{})
	tx.Where("collection_id = ?", t.Id).Delete(&model.AddressBookRule{})
	tx.Where("collection_id = ?", t.Id).Delete(&model.AddressBook{})
	tx.Delete(t)
	return tx.Commit().Error
}

func (s *AddressBookService) RuleInfoById(u uint) *model.AddressBookCollectionRule {
	p := &model.AddressBookCollectionRule{}
	DB.Where("id = ?", u).First(p)
	return p
}
func (s *AddressBookService) RulePersonalInfoByToIdAndCid(toid, cid uint) *model.AddressBookCollectionRule {
	return s.RuleInfoByToIdAndCid(model.ShareAddressBookRuleTypePersonal, toid, cid)
}
func (s *AddressBookService) RuleInfoByToIdAndCid(t int, toid, cid uint) *model.AddressBookCollectionRule {
	p := &model.AddressBookCollectionRule{}
	DB.Where("type = ? and to_id = ? and collection_id = ?", t, toid, cid).First(p)
	return p
}
func (s *AddressBookService) CreateRule(t *model.AddressBookCollectionRule) error {
	return DB.Create(t).Error
}

func (s *AddressBookService) ListRules(page uint, size uint, f func(tx *gorm.DB)) *model.AddressBookCollectionRuleList {
	res := &model.AddressBookCollectionRuleList{}
	res.Page = int64(page)
	res.PageSize = int64(size)
	tx := DB.Model(&model.AddressBookCollectionRule{})
	if f != nil {
		f(tx)
	}
	tx.Count(&res.Total)
	tx.Scopes(Paginate(page, size))
	tx.Find(&res.AddressBookCollectionRule)
	return res
}

func (s *AddressBookService) UpdateRule(t *model.AddressBookCollectionRule) error {
	return DB.Model(t).Select("*").Omit("created_at").Updates(t).Error
}

func (s *AddressBookService) DeleteRule(t *model.AddressBookCollectionRule) error {
	return DB.Delete(t).Error
}

func (s *AddressBookService) CollectionRulesForUser(user *model.User) (res []*model.AddressBookCollectionRule) {
	if user == nil || user.Id == 0 {
		return
	}
	var personalRules []*model.AddressBookCollectionRule
	DB.Where("type = ? and to_id = ?", model.ShareAddressBookRuleTypePersonal, user.Id).Find(&personalRules)
	res = append(res, personalRules...)

	if user.GroupId > 0 {
		var groupRules []*model.AddressBookCollectionRule
		DB.Where("type = ? and to_id = ?", model.ShareAddressBookRuleTypeGroup, user.GroupId).Find(&groupRules)
		res = append(res, groupRules...)
	}
	return
}

func (s *AddressBookService) AddressBookRulesForUser(user *model.User) (res []*model.AddressBookRule) {
	if user == nil || user.Id == 0 {
		return
	}
	var personalRules []*model.AddressBookRule
	DB.Where("type = ? and to_id = ?", model.ShareAddressBookRuleTypePersonal, user.Id).Find(&personalRules)
	res = append(res, personalRules...)

	if user.GroupId > 0 {
		var groupRules []*model.AddressBookRule
		DB.Where("type = ? and to_id = ?", model.ShareAddressBookRuleTypeGroup, user.GroupId).Find(&groupRules)
		res = append(res, groupRules...)
	}
	return
}

func (s *AddressBookService) AddressBookRulesForUserAndCollection(user *model.User, cid uint) (res []*model.AddressBookRule) {
	if user == nil || user.Id == 0 {
		return
	}
	var personalRules []*model.AddressBookRule
	DB.Where("type = ? and to_id = ? and collection_id = ?", model.ShareAddressBookRuleTypePersonal, user.Id, cid).Find(&personalRules)
	res = append(res, personalRules...)

	if user.GroupId > 0 {
		var groupRules []*model.AddressBookRule
		DB.Where("type = ? and to_id = ? and collection_id = ?", model.ShareAddressBookRuleTypeGroup, user.GroupId, cid).Find(&groupRules)
		res = append(res, groupRules...)
	}
	return
}

func (s *AddressBookService) AddressBookRuleInfoById(u uint) *model.AddressBookRule {
	p := &model.AddressBookRule{}
	DB.Where("id = ?", u).First(p)
	return p
}

func (s *AddressBookService) AddressBookRuleInfoByToIdAndRowId(t int, toid, rowID uint) *model.AddressBookRule {
	p := &model.AddressBookRule{}
	DB.Where("type = ? and to_id = ? and address_book_row_id = ?", t, toid, rowID).First(p)
	return p
}

func (s *AddressBookService) CreateAddressBookRule(t *model.AddressBookRule) error {
	return DB.Create(t).Error
}

func (s *AddressBookService) ListAddressBookRules(page uint, size uint, f func(tx *gorm.DB)) *model.AddressBookRuleList {
	res := &model.AddressBookRuleList{}
	res.Page = int64(page)
	res.PageSize = int64(size)
	tx := DB.Model(&model.AddressBookRule{})
	if f != nil {
		f(tx)
	}
	tx.Count(&res.Total)
	tx.Scopes(Paginate(page, size))
	tx.Find(&res.AddressBookRule)
	return res
}

func (s *AddressBookService) UpdateAddressBookRule(t *model.AddressBookRule) error {
	return DB.Model(t).Select("*").Omit("created_at").Updates(t).Error
}

func (s *AddressBookService) DeleteAddressBookRule(t *model.AddressBookRule) error {
	return DB.Delete(t).Error
}

// CheckCollectionOwner checks whether uid owns the collection.
func (s *AddressBookService) CheckCollectionOwner(uid uint, cid uint) bool {
	p := s.CollectionInfoById(cid)
	return p.UserId == uid
}

func (s *AddressBookService) BatchUpdateTags(abs []*model.AddressBook, tags []string) error {
	ids := make([]uint, 0)
	for _, ab := range abs {
		ids = append(ids, ab.RowId)
	}
	tagsv, _ := json.Marshal(tags)
	return DB.Model(&model.AddressBook{}).Where("row_id in ?", ids).Update("tags", tagsv).Error
}
