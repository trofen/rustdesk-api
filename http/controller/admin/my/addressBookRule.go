package my

import (
	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/http/request/admin"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
	"gorm.io/gorm"
)

type AddressBookRule struct {
}

func (abr *AddressBookRule) List(c *gin.Context) {
	query := &admin.AddressBookRuleQuery{}
	if err := c.ShouldBindQuery(query); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	u := service.AllService.UserService.CurUser(c)
	query.UserId = int(u.Id)

	res := service.AllService.AddressBookService.ListAddressBookRules(query.Page, query.PageSize, func(tx *gorm.DB) {
		tx.Where("user_id = ?", query.UserId)
		if query.CollectionId > 0 {
			tx.Where("collection_id = ?", query.CollectionId)
		}
		if query.AddressBookRowId > 0 {
			tx.Where("address_book_row_id = ?", query.AddressBookRowId)
		}
	})
	response.Success(c, res)
}

func (abr *AddressBookRule) Create(c *gin.Context) {
	f := &model.AddressBookRule{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	u := service.AllService.UserService.CurUser(c)
	f.UserId = u.Id

	errList := global.Validator.ValidStruct(c, f)
	if len(errList) > 0 {
		response.Fail(c, 101, errList[0])
		return
	}
	if f.Type != model.ShareAddressBookRuleTypePersonal && f.Type != model.ShareAddressBookRuleTypeGroup {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	msg, ok := abr.CheckForm(u, f)
	if !ok {
		response.Fail(c, 101, response.TranslateMsg(c, msg))
		return
	}
	if err := service.AllService.AddressBookService.CreateAddressBookRule(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed")+err.Error())
		return
	}
	response.Success(c, nil)
}

func (abr *AddressBookRule) CheckForm(u *model.User, t *model.AddressBookRule) (string, bool) {
	if t.UserId != u.Id || t.CollectionId == 0 {
		return "NoAccess", false
	}
	ab := service.AllService.AddressBookService.InfoByRowId(t.AddressBookRowId)
	if ab == nil || ab.RowId == 0 {
		return "ItemNotFound", false
	}
	if ab.UserId != u.Id || ab.CollectionId != t.CollectionId || ab.CollectionId == 0 {
		return "ParamsError", false
	}
	if !service.AllService.AddressBookService.CheckCollectionOwner(t.UserId, t.CollectionId) {
		return "ParamsError", false
	}

	if t.Type == model.ShareAddressBookRuleTypePersonal {
		if t.ToId == t.UserId {
			return "CannotShareToSelf", false
		}
		tou := service.AllService.UserService.InfoById(t.ToId)
		if tou.Id == 0 {
			return "ItemNotFound", false
		}
	} else if t.Type == model.ShareAddressBookRuleTypeGroup {
		tog := service.AllService.GroupService.InfoById(t.ToId)
		if tog.Id == 0 {
			return "ItemNotFound", false
		}
	} else {
		return "ParamsError", false
	}

	ex := service.AllService.AddressBookService.AddressBookRuleInfoByToIdAndRowId(t.Type, t.ToId, t.AddressBookRowId)
	if t.Id == 0 && ex.Id > 0 {
		return "ItemExists", false
	}
	if t.Id > 0 && ex.Id > 0 && t.Id != ex.Id {
		return "ItemExists", false
	}
	return "", true
}

func (abr *AddressBookRule) Update(c *gin.Context) {
	f := &model.AddressBookRule{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	if f.Id == 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	u := service.AllService.UserService.CurUser(c)
	ex := service.AllService.AddressBookService.AddressBookRuleInfoById(f.Id)
	if ex.Id == 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "ItemNotFound"))
		return
	}
	if ex.UserId != u.Id {
		response.Fail(c, 101, response.TranslateMsg(c, "NoAccess"))
		return
	}
	f.UserId = u.Id

	errList := global.Validator.ValidStruct(c, f)
	if len(errList) > 0 {
		response.Fail(c, 101, errList[0])
		return
	}
	msg, ok := abr.CheckForm(u, f)
	if !ok {
		response.Fail(c, 101, response.TranslateMsg(c, msg))
		return
	}
	if err := service.AllService.AddressBookService.UpdateAddressBookRule(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed")+err.Error())
		return
	}
	response.Success(c, nil)
}

func (abr *AddressBookRule) Delete(c *gin.Context) {
	f := &model.AddressBookRule{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	errList := global.Validator.ValidVar(c, f.Id, "required,gt=0")
	if len(errList) > 0 {
		response.Fail(c, 101, errList[0])
		return
	}
	ex := service.AllService.AddressBookService.AddressBookRuleInfoById(f.Id)
	if ex.Id == 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "ItemNotFound"))
		return
	}
	u := service.AllService.UserService.CurUser(c)
	if ex.UserId != u.Id {
		response.Fail(c, 101, response.TranslateMsg(c, "NoAccess"))
		return
	}
	if err := service.AllService.AddressBookService.DeleteAddressBookRule(ex); err == nil {
		response.Success(c, nil)
		return
	}
	response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed"))
}
