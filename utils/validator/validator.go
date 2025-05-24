package validator

import (
	"errors"
	"reflect"
	"regexp"
	"strings"

	"github.com/farnese17/chat/utils/errorsx"
	"github.com/farnese17/chat/utils/logger"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	zh_trans "github.com/go-playground/validator/v10/translations/zh"
)

var (
	validate *validator.Validate
	trans    ut.Translator
)

func SetupValidator() {
	validate = validator.New()
	zh_ch := zh.New()
	uni := ut.New(zh_ch)
	trans, _ = uni.GetTranslator("zh")

	registerCustomValidations(validate, trans)

	zh_trans.RegisterDefaultTranslations(validate, trans)
	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		label := field.Tag.Get("label")
		return label
	})
}

func registerCustomValidations(_ *validator.Validate, trans ut.Translator) {
	validate.RegisterValidation("pwlength", PasswordLength)
	validate.RegisterTranslation("pwlength", trans, func(ut ut.Translator) error {
		return ut.Add("pwlength", "密码长度应该在6到16个字符之间", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("pwlength", fe.Field())
		return t
	})
	validate.RegisterValidation("nospace", NoSpace)
	validate.RegisterTranslation("nospace", trans, func(ut ut.Translator) error {
		return ut.Add("nospace", "密码不能包含空格", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("nospace", fe.Field())
		return t
	})

	// 自定义验证手机号码
	validate.RegisterValidation("mobile", Mobile)
	validate.RegisterTranslation("mobile", trans, func(ut ut.Translator) error {
		return ut.Add("mobile", "请输入正确的手机号", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("mobile", fe.Field())
		return t
	})
	// 正则验证邮箱
	validate.RegisterValidation("email", Email)
	validate.RegisterTranslation("email", trans, func(ut ut.Translator) error {
		return ut.Add("email", "请输入正确的邮箱地址", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("email", fe.Field())
		return t
	})
	// 自定义验证uid
	validate.RegisterValidation("uid", UID)
	validate.RegisterTranslation("uid", trans, func(ut ut.Translator) error {
		return ut.Add("uid", "无效参数", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("uid", fe.Field())
		return t
	})
	// 自定义验证gid
	validate.RegisterValidation("gid", GID)
	validate.RegisterTranslation("gid", trans, func(ut ut.Translator) error {
		return ut.Add("gid", "无效参数", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("gid", fe.Field())
		return t
	})
	// 验证用户名
	validate.RegisterValidation("username", UserName)
	validate.RegisterTranslation("username", trans, func(ut ut.Translator) error {
		return ut.Add("username", "用户名不能为空", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("username", fe.Field())
		return t
	})
}

func Validate(data interface{}) error {
	err := validate.Struct(data)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			return errors.New(err.Translate(trans))
		}
	}
	return nil
}

func validateVar(data interface{}, tag string) string {
	err := validate.Var(data, tag)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			return err.Translate(trans)
		}
	}
	return ""
}

func Mobile(fl validator.FieldLevel) bool {
	phone := fl.Field().String()
	if m, _ := regexp.MatchString(`^[1][3-9][0-9]{9}$`, phone); !m {
		return false
	}
	return true
}

func Email(fl validator.FieldLevel) bool {
	email := fl.Field().String()
	if m, _ := regexp.MatchString(`^[a-zA-Z0-9]+([._-][a-zA-Z0-9]+)*@[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*(\.[a-zA-Z0-9]{2,})+$`, email); !m {
		return false
	}
	return true
}

func PasswordLength(fl validator.FieldLevel) bool {
	pw := fl.Field().String()
	return len(pw) >= 6 && len(pw) <= 16
}

func NoSpace(fl validator.FieldLevel) bool {
	pw := fl.Field().String()
	for _, ch := range pw {
		if ch == ' ' {
			return false
		}
	}
	return true
}

func UID(fl validator.FieldLevel) bool {
	id := fl.Field().Uint()
	return id > uint64(1e5) && id < uint64(1e9)
}

func GID(fl validator.FieldLevel) bool {
	gid := fl.Field().Uint()
	return gid > uint64(1e9)
}

func UserName(fl validator.FieldLevel) bool {
	name := fl.Field().String()
	return strings.TrimSpace(name) != ""
}

func ValidateMobile(phone string) error {
	if err := validateVar(phone, "mobile"); err != "" {
		return errors.New(err)
	}
	return nil
}

func ValidateUID(id uint) error {
	if err := validateVar(id, "uid"); err != "" {
		return errors.New(err)
	}
	return nil
}

func ValidateEmail(email string) error {
	if err := validateVar(email, "email"); err != "" {
		return errors.New(err)
	}
	return nil
}

func ValidateGID(gid uint) error {
	if err := validateVar(gid, "gid"); err != "" {
		return errors.New(err)
	}
	return nil
}

func ValidateUsername(name string) error {
	if err := validateVar(name, "required,min=2,max=8,username"); err != "" {
		return errors.New("用户名" + err)
	}
	return nil
}
func ValidatePassword(password string) error {
	if err := validateVar(password, "pwlength,nospace"); err != "" {
		return errors.New(err)
	}
	return nil
}

func ValidateGroupname(name string) error {
	if err := validateVar(name, "max=20"); err != "" {
		return errors.New("群组名称" + err)
	}
	return nil
}

func VaildateGroupDesc(desc string) error {
	if err := validateVar(desc, "max=255"); err != "" {
		return errors.New("群组描述" + err)
	}
	return nil
}

func ValidateRemark(remark string) error {
	if err := validateVar(remark, "max=8"); err != "" {
		return errors.New("备注" + err)
	}
	return nil
}

func ValidateGIDAndUID(gid uint, uid ...uint) error {
	if err := ValidateGID(gid); err != nil {
		return err
	}
	for _, id := range uid {
		if err := ValidateUID(id); err != nil {
			return err
		}
	}
	return nil
}

func VerfityPageSize(pagesize int) error {
	if pagesize < 1 || pagesize > 30 {
		logger := logger.GetLogger()
		logger.Warn("page size invalid", zap.Int("pagesize", pagesize))
		if pagesize < 1 {
			return errorsx.ErrPageSizeTooSmall
		}
		return errorsx.ErrPageSizeTooBig
	}
	return nil
}
