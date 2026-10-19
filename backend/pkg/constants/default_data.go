package constants

import (
	"time"

	"github.com/tx7do/go-utils/timeutil"
	"github.com/tx7do/go-utils/trans"

	authenticationV1 "go-wind-ops/api/gen/go/authentication/service/v1"
	dictV1 "go-wind-ops/api/gen/go/dict/service/v1"
	identityV1 "go-wind-ops/api/gen/go/identity/service/v1"
	permissionV1 "go-wind-ops/api/gen/go/permission/service/v1"
)

const (
	// DefaultAdminUserName 系统初始化默认管理员用户名
	DefaultAdminUserName = "admin"
	// DefaultAdminPassword 系统初始化默认管理员用户密码
	DefaultAdminPassword = "admin"

	// DefaultUserPassword 系统初始化默认普通用户密码
	DefaultUserPassword = "12345678"

	// PlatformTenantID 平台管理员租户ID
	PlatformTenantID = uint32(0)
)

// DefaultPermissionGroups 系统初始化默认权限组数据
var DefaultPermissionGroups = []*permissionV1.PermissionGroup{
	{
		//Id:        trans.Ptr(uint32(1)),
		Name:      trans.Ptr("系统管理"),
		Path:      trans.Ptr("/"),
		Module:    trans.Ptr(SystemPermissionModule),
		SortOrder: trans.Ptr(uint32(1)),
		Status:    trans.Ptr(permissionV1.PermissionGroup_ON),
	},
	{
		//Id:        trans.Ptr(uint32(2)),
		ParentId:  trans.Ptr(uint32(1)),
		Name:      trans.Ptr("系统权限"),
		Path:      trans.Ptr("/1/2/"),
		Module:    trans.Ptr(SystemPermissionModule),
		SortOrder: trans.Ptr(uint32(1)),
		Status:    trans.Ptr(permissionV1.PermissionGroup_ON),
	},
	{
		//Id:        trans.Ptr(uint32(3)),
		ParentId:  trans.Ptr(uint32(1)),
		Name:      trans.Ptr("租户管理"),
		Path:      trans.Ptr("/1/3/"),
		Module:    trans.Ptr(SystemPermissionModule),
		SortOrder: trans.Ptr(uint32(2)),
		Status:    trans.Ptr(permissionV1.PermissionGroup_ON),
	},
	{
		//Id:        trans.Ptr(uint32(4)),
		ParentId:  trans.Ptr(uint32(1)),
		Name:      trans.Ptr("审计管理"),
		Path:      trans.Ptr("/1/4/"),
		Module:    trans.Ptr(SystemPermissionModule),
		SortOrder: trans.Ptr(uint32(3)),
		Status:    trans.Ptr(permissionV1.PermissionGroup_ON),
	},
	{
		//Id:        trans.Ptr(uint32(5)),
		ParentId:  trans.Ptr(uint32(1)),
		Name:      trans.Ptr("安全策略"),
		Path:      trans.Ptr("/1/5/"),
		Module:    trans.Ptr(SystemPermissionModule),
		SortOrder: trans.Ptr(uint32(4)),
		Status:    trans.Ptr(permissionV1.PermissionGroup_ON),
	},
}

// DefaultPermissions 系统初始化默认权限数据
var DefaultPermissions = []*permissionV1.Permission{
	{
		//Id:          trans.Ptr(uint32(1)),
		GroupId:     trans.Ptr(uint32(2)),
		Name:        trans.Ptr("访问后台"),
		Description: trans.Ptr("允许用户访问系统后台管理界面"),
		Code:        trans.Ptr(SystemAccessBackendPermissionCode),
		Status:      trans.Ptr(permissionV1.Permission_ON),
	},
	{
		//Id:          trans.Ptr(uint32(2)),
		GroupId:     trans.Ptr(uint32(2)),
		Name:        trans.Ptr("平台管理员权限"),
		Description: trans.Ptr("拥有系统所有功能的操作权限，可管理租户、用户、角色及所有资源"),
		Code:        trans.Ptr(SystemPlatformAdminPermissionCode),
		Status:      trans.Ptr(permissionV1.Permission_ON),
		MenuIds: []uint32{
			1, 2, 3, 4, 5, 6,
			10, 11,
			20, 21, 22, 23, 24,
			30, 31, 32,
			40, 41, 42,
			50, 51, 52, 53, 54, 55,
			60, 61, 62, 63, 64, 65, 66, 67,
		},
		ApiIds: []uint32{
			1, 2, 3, 4, 5, 6, 7, 8, 9,
			10, 11, 12, 13, 14, 15, 16, 17, 18, 19,
			20, 21, 22, 23, 24, 25, 26, 27, 28, 29,
			30, 31, 32, 33, 34, 35, 36, 37, 38, 39,
			40, 41, 42, 43, 44, 45, 46, 47, 48, 49,
			50, 51, 52, 53, 54, 55, 56, 57, 58, 59,
			60, 61, 62, 63, 64, 65, 66, 67, 68,
			70, 71, 72, 73, 74, 75, 76, 77, 78, 79,
			80, 81, 82, 83, 84, 85, 86, 87, 88, 89,
			90, 91, 92, 93, 94, 95, 96, 97, 98, 99,
			100, 101, 102, 103, 104, 105, 106, 107, 108, 109,
			110, 111, 112, 113, 114, 115, 116, 117, 118, 119,
			120, 121, 122, 123, 124, 125, 126, 127, 128, 129,
			130, 131, 132, 133, 134, 135, 136,
		},
	},
	{
		//Id:          trans.Ptr(uint32(3)),
		GroupId:     trans.Ptr(uint32(3)),
		Name:        trans.Ptr("租户管理员权限"),
		Description: trans.Ptr("拥有租户内所有功能的操作权限，可管理用户、角色及租户内所有资源"),
		Code:        trans.Ptr(SystemTenantManagerPermissionCode),
		Status:      trans.Ptr(permissionV1.Permission_ON),
		MenuIds: []uint32{
			1, 2,
			20, 21, 22, 23, 24,
			30, 32,
			40, 41,
			50, 51,
			60, 61, 62, 63, 64,
		},
		ApiIds: []uint32{
			1, 2, 3, 4, 5, 6, 7, 8, 9,
			10, 11, 12, 13, 14, 15, 16, 17, 18, 19,
			20, 21, 22, 23, 24, 25, 26, 27, 28, 29,
			30, 31, 32, 33, 34, 35, 36, 37, 38, 39,
			40, 41, 42, 43, 44, 45, 46, 47, 48, 49,
			50, 51, 52, 53, 54, 55, 56, 57, 58, 59,
			60, 61, 62, 63, 64, 65, 66, 67, 68,
			70, 71, 72, 73, 74, 75, 76, 77, 78, 79,
			80, 81, 82, 83, 84, 85, 86, 87, 88, 89,
			90, 91, 92, 93, 94, 95, 96, 97, 98, 99,
			100, 101, 102, 103, 104, 105, 106, 107, 108, 109,
			110, 111, 112, 113, 114, 115, 116, 117, 118, 119,
			120, 121, 122, 123, 124, 125, 126, 127, 128,
		},
	},

	{
		//Id:          trans.Ptr(uint32(4)),
		GroupId:     trans.Ptr(uint32(3)),
		Name:        trans.Ptr("管理租户"),
		Description: trans.Ptr("允许创建/修改/删除租户"),
		Code:        trans.Ptr(SystemManageTenantsPermissionCode),
		Status:      trans.Ptr(permissionV1.Permission_ON),
	},
	{
		//Id:          trans.Ptr(uint32(5)),
		GroupId:     trans.Ptr(uint32(4)),
		Name:        trans.Ptr("查看审计日志"),
		Description: trans.Ptr("允许查看系统操作日志"),
		Code:        trans.Ptr(SystemAuditLogsPermissionCode),
		Status:      trans.Ptr(permissionV1.Permission_ON),
	},
}

// DefaultRoles 系统初始化默认角色数据
var DefaultRoles = []*permissionV1.Role{
	{
		//Id:          trans.Ptr(uint32(1)),
		Name:        trans.Ptr(DefaultPlatformAdminRoleName),
		Code:        trans.Ptr(PlatformAdminRoleCode),
		Status:      trans.Ptr(permissionV1.Role_ON),
		Description: trans.Ptr("拥有系统所有功能的操作权限，可管理租户、用户、角色及所有资源"),
		IsProtected: trans.Ptr(true),
		Type:        trans.Ptr(permissionV1.Role_SYSTEM),
		SortOrder:   trans.Ptr(uint32(1)),
		Permissions: []uint32{1, 2, 4},
	},
	{
		//Id:          trans.Ptr(uint32(2)),
		Name:        trans.Ptr(DefaultTenantManagerRoleName + "模板"),
		Code:        trans.Ptr(TenantAdminTemplateRoleCode),
		Status:      trans.Ptr(permissionV1.Role_ON),
		Description: trans.Ptr("租户管理员角色，拥有租户内所有功能的操作权限，可管理用户、角色及租户内所有资源"),
		IsProtected: trans.Ptr(true),
		Type:        trans.Ptr(permissionV1.Role_TEMPLATE),
		SortOrder:   trans.Ptr(uint32(2)),
		Permissions: []uint32{1, 3},
	},
}

// DefaultRoleMetadata 系统初始化默认角色元数据
var DefaultRoleMetadata = []*permissionV1.RoleMetadata{
	{
		//Id:              trans.Ptr(uint32(1)),
		RoleId:          trans.Ptr(uint32(1)),
		IsTemplate:      trans.Ptr(false),
		TemplateVersion: trans.Ptr(int32(1)),
		Scope:           permissionV1.RoleMetadata_PLATFORM.Enum(),
		SyncPolicy:      permissionV1.RoleMetadata_AUTO.Enum(),
	},
	{
		//Id:              trans.Ptr(uint32(2)),
		RoleId:          trans.Ptr(uint32(2)),
		IsTemplate:      trans.Ptr(true),
		TemplateFor:     trans.Ptr(TenantAdminRoleCode),
		TemplateVersion: trans.Ptr(int32(1)),
		Scope:           permissionV1.RoleMetadata_TENANT.Enum(),
		SyncPolicy:      permissionV1.RoleMetadata_AUTO.Enum(),
	},
}

// DefaultUsers 系统初始化默认用户数据
var DefaultUsers = []*identityV1.User{
	{
		//Id:       trans.Ptr(uint32(1)),
		TenantId: trans.Ptr(uint32(0)),
		Username: trans.Ptr(DefaultAdminUserName),
		Realname: trans.Ptr("喵个咪"),
		Nickname: trans.Ptr("鹳狸猿"),
		Region:   trans.Ptr("中国"),
		Email:    trans.Ptr("admin@gmail.com"),
	},
}

// DefaultUserCredentials 系统初始化默认用户凭证数据
var DefaultUserCredentials = []*authenticationV1.UserCredential{
	{
		UserId:         trans.Ptr(uint32(1)),
		TenantId:       trans.Ptr(uint32(0)),
		IdentityType:   authenticationV1.UserCredential_USERNAME.Enum(),
		Identifier:     trans.Ptr(DefaultAdminUserName),
		CredentialType: authenticationV1.UserCredential_PASSWORD_HASH.Enum(),
		Credential:     trans.Ptr(DefaultAdminPassword),
		IsPrimary:      trans.Ptr(true),
		Status:         authenticationV1.UserCredential_ENABLED.Enum(),
	},
}

// DefaultUserRoles 系统初始化默认用户角色关系数据
var DefaultUserRoles = []*permissionV1.UserRole{
	{
		UserId:    trans.Ptr(uint32(1)),
		TenantId:  trans.Ptr(uint32(0)),
		RoleId:    trans.Ptr(uint32(1)),
		IsPrimary: trans.Ptr(true),
		Status:    permissionV1.UserRole_ACTIVE.Enum(),
	},
}

// DefaultMemberships 系统初始化默认用户成员关系数据
var DefaultMemberships = []*identityV1.Membership{
	{
		UserId:    trans.Ptr(uint32(1)),
		TenantId:  trans.Ptr(uint32(0)),
		Status:    identityV1.Membership_ACTIVE.Enum(),
		IsPrimary: trans.Ptr(true),
		RoleIds:   []uint32{1},
	},
}

// DefaultLanguages 系统初始化默认语言数据
var DefaultLanguages = []*dictV1.Language{
	{LanguageCode: trans.Ptr("zh-CN"), LanguageName: trans.Ptr("中文（简体）"), NativeName: trans.Ptr("简体中文"), IsDefault: trans.Ptr(true), IsEnabled: trans.Ptr(true), SortOrder: trans.Uint32(0)},
	{LanguageCode: trans.Ptr("zh-TW"), LanguageName: trans.Ptr("中文（繁体）"), NativeName: trans.Ptr("繁體中文"), IsDefault: trans.Ptr(false), IsEnabled: trans.Ptr(true), SortOrder: trans.Uint32(100)},
	{LanguageCode: trans.Ptr("en-US"), LanguageName: trans.Ptr("英语"), NativeName: trans.Ptr("English"), IsDefault: trans.Ptr(false), IsEnabled: trans.Ptr(true), SortOrder: trans.Uint32(1)},
	{LanguageCode: trans.Ptr("ja-JP"), LanguageName: trans.Ptr("日语"), NativeName: trans.Ptr("日本語"), IsDefault: trans.Ptr(false), IsEnabled: trans.Ptr(true), SortOrder: trans.Uint32(100)},
	{LanguageCode: trans.Ptr("ko-KR"), LanguageName: trans.Ptr("韩语"), NativeName: trans.Ptr("한국어"), IsDefault: trans.Ptr(false), IsEnabled: trans.Ptr(true), SortOrder: trans.Uint32(100)},
	{LanguageCode: trans.Ptr("es-ES"), LanguageName: trans.Ptr("西班牙语"), NativeName: trans.Ptr("Español"), IsDefault: trans.Ptr(false), IsEnabled: trans.Ptr(true), SortOrder: trans.Uint32(100)},
	{LanguageCode: trans.Ptr("fr-FR"), LanguageName: trans.Ptr("法语"), NativeName: trans.Ptr("Français"), IsDefault: trans.Ptr(false), IsEnabled: trans.Ptr(true), SortOrder: trans.Uint32(100)},
}

// DefaultOpsPermissions 运维域权限码(id 5-12,组 6 运维管理)。
// 全部挂到运维菜单(id 80-96);平台管理员角色默认绑定。
var DefaultOpsPermissions = []*permissionV1.Permission{
	{Id: trans.Ptr(uint32(10)), GroupId: trans.Ptr(uint32(6)), Name: trans.Ptr("节点管理"), Description: trans.Ptr("节点查看/指令台账/文件管理/终端/插件"), Code: trans.Ptr(OpsNodeManagePermissionCode), Status: trans.Ptr(permissionV1.Permission_ON), MenuIds: opsMenuIds()},
	{Id: trans.Ptr(uint32(11)), GroupId: trans.Ptr(uint32(6)), Name: trans.Ptr("项目管理"), Description: trans.Ptr("受管项目管理"), Code: trans.Ptr(OpsProjectManagePermissionCode), Status: trans.Ptr(permissionV1.Permission_ON), MenuIds: opsMenuIds()},
	{Id: trans.Ptr(uint32(12)), GroupId: trans.Ptr(uint32(6)), Name: trans.Ptr("项目部署"), Description: trans.Ptr("项目启停/重启"), Code: trans.Ptr(OpsProjectDeployPermissionCode), Status: trans.Ptr(permissionV1.Permission_ON), MenuIds: opsMenuIds()},
	{Id: trans.Ptr(uint32(13)), GroupId: trans.Ptr(uint32(6)), Name: trans.Ptr("脚本管理"), Description: trans.Ptr("运维脚本管理"), Code: trans.Ptr(OpsScriptManagePermissionCode), Status: trans.Ptr(permissionV1.Permission_ON), MenuIds: opsMenuIds()},
	{Id: trans.Ptr(uint32(14)), GroupId: trans.Ptr(uint32(6)), Name: trans.Ptr("脚本执行"), Description: trans.Ptr("远程脚本执行"), Code: trans.Ptr(OpsScriptExecutePermissionCode), Status: trans.Ptr(permissionV1.Permission_ON), MenuIds: opsMenuIds()},
	{Id: trans.Ptr(uint32(15)), GroupId: trans.Ptr(uint32(6)), Name: trans.Ptr("构建管理"), Description: trans.Ptr("仓库/构建/流水线管理"), Code: trans.Ptr(OpsBuildManagePermissionCode), Status: trans.Ptr(permissionV1.Permission_ON), MenuIds: opsMenuIds()},
	{Id: trans.Ptr(uint32(16)), GroupId: trans.Ptr(uint32(6)), Name: trans.Ptr("构建触发"), Description: trans.Ptr("触发构建/流水线"), Code: trans.Ptr(OpsBuildTriggerPermissionCode), Status: trans.Ptr(permissionV1.Permission_ON), MenuIds: opsMenuIds()},
	{Id: trans.Ptr(uint32(17)), GroupId: trans.Ptr(uint32(6)), Name: trans.Ptr("产物分发"), Description: trans.Ptr("构建产物分发"), Code: trans.Ptr(OpsDistributePermissionCode), Status: trans.Ptr(permissionV1.Permission_ON), MenuIds: opsMenuIds()},
}

// opsMenuIds 运维菜单 ID 集合(81-85 一级分组 + 86-108 页面)。
func opsMenuIds() []uint32 {
	ids := make([]uint32, 0, 28)
	for i := uint32(81); i <= 108; i++ {
		ids = append(ids, i)
	}
	return ids
}

// DefaultMenus 系统初始化默认菜单数据
// DefaultMenus 系统初始化默认菜单数据
var DefaultMenus = []*permissionV1.Menu{
	{
		Id:        trans.Ptr(uint32(1)),
		ParentId:  nil,
		Type:      permissionV1.Menu_CATALOG.Enum(),
		Name:      trans.Ptr("Dashboard"),
		Path:      trans.Ptr("/dashboard"),
		Component: trans.Ptr("BasicLayout"),
		Status:    permissionV1.Menu_ON.Enum(),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Order:     trans.Ptr(int32(-1)),
			Title:     trans.Ptr("page.dashboard.title"),
			Icon:      trans.Ptr("lucide:layout-dashboard"),
			Authority: []string{"sys:platform_admin", "sys:tenant_manager"},
		},
	},

	{
		Id:        trans.Ptr(uint32(3)),
		ParentId:  nil,
		Type:      permissionV1.Menu_CATALOG.Enum(),
		Name:      trans.Ptr("Profile"),
		Path:      trans.Ptr("/profile"),
		Component: trans.Ptr("BasicLayout"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Title:      trans.Ptr("menu.profile.settings"),
			HideInMenu: trans.Ptr(true),
		},
	},
	{
		Id:        trans.Ptr(uint32(4)),
		ParentId:  trans.Ptr(uint32(3)),
		Type:      permissionV1.Menu_MENU.Enum(),
		Name:      trans.Ptr("ProfilePage"),
		Path:      trans.Ptr("/profile"),
		Component: trans.Ptr("profile/index.vue"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Title:      trans.Ptr("menu.profile.settings"),
			Icon:       trans.Ptr("lucide:user-pen"),
			HideInMenu: trans.Ptr(true),
		},
	},


	{
		Id:        trans.Ptr(uint32(10)),
		ParentId:  nil,
		Type:      permissionV1.Menu_CATALOG.Enum(),
		Name:      trans.Ptr("TenantManagement"),
		Path:      trans.Ptr("/tenant"),
		Redirect:  trans.Ptr("/tenant/members"),
		Component: trans.Ptr("BasicLayout"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Order:     trans.Ptr(int32(2000)),
			Title:     trans.Ptr("menu.tenant.moduleName"),
			Icon:      trans.Ptr("lucide:building-2"),
			Authority: []string{"sys:platform_admin"},
		},
	},
	{
		Id:        trans.Ptr(uint32(11)),
		ParentId:  trans.Ptr(uint32(10)),
		Type:      permissionV1.Menu_MENU.Enum(),
		Name:      trans.Ptr("TenantMemberManagement"),
		Path:      trans.Ptr("members"),
		Component: trans.Ptr("app/tenant/tenant/index.vue"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Order:     trans.Ptr(int32(1)),
			Title:     trans.Ptr("menu.tenant.member"),
			Icon:      trans.Ptr("lucide:users"),
			Authority: []string{"sys:platform_admin"},
			AffixTab:  trans.Ptr(true),
		},
	},

	{
		Id:        trans.Ptr(uint32(20)),
		ParentId:  nil,
		Type:      permissionV1.Menu_CATALOG.Enum(),
		Name:      trans.Ptr("OrganizationalPersonnelManagement"),
		Path:      trans.Ptr("/opm"),
		Redirect:  trans.Ptr("/opm/users"),
		Component: trans.Ptr("BasicLayout"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Order:     trans.Ptr(int32(2001)),
			Title:     trans.Ptr("menu.opm.moduleName"),
			Icon:      trans.Ptr("lucide:users"),
			KeepAlive: trans.Ptr(true),
			Authority: []string{"sys:platform_admin", "sys:tenant_manager"},
		},
	},
	{
		Id:        trans.Ptr(uint32(21)),
		ParentId:  trans.Ptr(uint32(20)),
		Type:      permissionV1.Menu_MENU.Enum(),
		Name:      trans.Ptr("OrgUnitManagement"),
		Path:      trans.Ptr("org-units"),
		Component: trans.Ptr("app/opm/org_unit/index.vue"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Order:     trans.Ptr(int32(1)),
			Title:     trans.Ptr("menu.opm.orgUnit"),
			Icon:      trans.Ptr("lucide:layers"),
			Authority: []string{"sys:platform_admin", "sys:tenant_manager"},
		},
	},
	{
		Id:        trans.Ptr(uint32(22)),
		ParentId:  trans.Ptr(uint32(20)),
		Type:      permissionV1.Menu_MENU.Enum(),
		Name:      trans.Ptr("PositionManagement"),
		Path:      trans.Ptr("positions"),
		Component: trans.Ptr("app/opm/position/index.vue"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Order:     trans.Ptr(int32(2)),
			Title:     trans.Ptr("menu.opm.position"),
			Icon:      trans.Ptr("lucide:briefcase"),
			Authority: []string{"sys:platform_admin", "sys:tenant_manager"},
		},
	},
	{
		Id:        trans.Ptr(uint32(23)),
		ParentId:  trans.Ptr(uint32(20)),
		Type:      permissionV1.Menu_MENU.Enum(),
		Name:      trans.Ptr("UserManagement"),
		Path:      trans.Ptr("users"),
		Component: trans.Ptr("app/opm/position/index.vue"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Order:     trans.Ptr(int32(3)),
			Title:     trans.Ptr("menu.opm.user"),
			Icon:      trans.Ptr("lucide:user"),
			Authority: []string{"sys:platform_admin", "sys:tenant_manager"},
		},
	},
	{
		Id:        trans.Ptr(uint32(24)),
		ParentId:  trans.Ptr(uint32(20)),
		Type:      permissionV1.Menu_MENU.Enum(),
		Name:      trans.Ptr("UserDetail"),
		Path:      trans.Ptr("users/detail/:id"),
		Component: trans.Ptr("app/opm/user/detail/index.vue"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Title:      trans.Ptr("menu.opm.userDetail"),
			Authority:  []string{"sys:platform_admin", "sys:tenant_manager"},
			HideInMenu: trans.Ptr(true),
		},
	},

	{
		Id:        trans.Ptr(uint32(30)),
		ParentId:  nil,
		Type:      permissionV1.Menu_CATALOG.Enum(),
		Name:      trans.Ptr("PermissionManagement"),
		Path:      trans.Ptr("/permission"),
		Redirect:  trans.Ptr("/permission/codes"),
		Component: trans.Ptr("BasicLayout"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Order:     trans.Ptr(int32(2002)),
			Title:     trans.Ptr("menu.permission.moduleName"),
			Icon:      trans.Ptr("lucide:shield-check"),
			KeepAlive: trans.Ptr(true),
			Authority: []string{"sys:platform_admin", "sys:tenant_manager"},
		},
	},
	{
		Id:        trans.Ptr(uint32(31)),
		ParentId:  trans.Ptr(uint32(30)),
		Type:      permissionV1.Menu_MENU.Enum(),
		Name:      trans.Ptr("PermissionPointManagement"),
		Path:      trans.Ptr("codes"),
		Component: trans.Ptr("app/permission/permission/index.vue"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Title:     trans.Ptr("menu.permission.permission"),
			Icon:      trans.Ptr("lucide:shield-ellipsis"),
			Order:     trans.Ptr(int32(1)),
			Authority: []string{"sys:platform_admin"},
		},
	},
	{
		Id:        trans.Ptr(uint32(32)),
		ParentId:  trans.Ptr(uint32(30)),
		Type:      permissionV1.Menu_MENU.Enum(),
		Name:      trans.Ptr("RoleManagement"),
		Path:      trans.Ptr("roles"),
		Component: trans.Ptr("app/permission/role/index.vue"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Title:     trans.Ptr("menu.permission.role"),
			Icon:      trans.Ptr("lucide:shield-user"),
			Order:     trans.Ptr(int32(2)),
			Authority: []string{"sys:platform_admin", "sys:tenant_manager"},
		},
	},


	{
		Id:        trans.Ptr(uint32(50)),
		ParentId:  nil,
		Type:      permissionV1.Menu_CATALOG.Enum(),
		Name:      trans.Ptr("LogAuditManagement"),
		Path:      trans.Ptr("/log"),
		Redirect:  trans.Ptr("/log/login-audit-logs"),
		Component: trans.Ptr("BasicLayout"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Order:     trans.Ptr(int32(2004)),
			Title:     trans.Ptr("menu.log.moduleName"),
			Icon:      trans.Ptr("lucide:logs"),
			KeepAlive: trans.Ptr(true),
			Authority: []string{"sys:platform_admin"},
		},
	},
	{
		Id:        trans.Ptr(uint32(51)),
		ParentId:  trans.Ptr(uint32(50)),
		Type:      permissionV1.Menu_MENU.Enum(),
		Name:      trans.Ptr("LoginAuditLog"),
		Path:      trans.Ptr("login-audit-logs"),
		Component: trans.Ptr("app/log/login_audit_log/index.vue"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Title:     trans.Ptr("menu.log.loginAuditLog"),
			Icon:      trans.Ptr("lucide:user-lock"),
			Order:     trans.Ptr(int32(1)),
			Authority: []string{"sys:platform_admin"},
		},
	},
	{
		Id:        trans.Ptr(uint32(52)),
		ParentId:  trans.Ptr(uint32(50)),
		Type:      permissionV1.Menu_MENU.Enum(),
		Name:      trans.Ptr("ApiAuditLog"),
		Path:      trans.Ptr("api-audit-logs"),
		Component: trans.Ptr("app/log/api_audit_log/index.vue"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Title:     trans.Ptr("menu.log.apiAuditLog"),
			Icon:      trans.Ptr("lucide:file-clock"),
			Order:     trans.Ptr(int32(2)),
			Authority: []string{"sys:platform_admin"},
		},
	},
	{
		Id:        trans.Ptr(uint32(53)),
		ParentId:  trans.Ptr(uint32(50)),
		Type:      permissionV1.Menu_MENU.Enum(),
		Name:      trans.Ptr("OperationAuditLog"),
		Path:      trans.Ptr("operation-audit-logs"),
		Component: trans.Ptr("app/log/operation_audit_log/index.vue"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Title:     trans.Ptr("menu.log.operationAuditLog"),
			Icon:      trans.Ptr("lucide:shield-ellipsis"),
			Order:     trans.Ptr(int32(3)),
			Authority: []string{"sys:platform_admin"},
		},
	},
	{
		Id:        trans.Ptr(uint32(54)),
		ParentId:  trans.Ptr(uint32(50)),
		Type:      permissionV1.Menu_MENU.Enum(),
		Name:      trans.Ptr("DataAccessAuditLog"),
		Path:      trans.Ptr("data-access-audit-logs"),
		Component: trans.Ptr("app/log/data_access_audit_log/index.vue"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Title:     trans.Ptr("menu.log.dataAccessAuditLog"),
			Icon:      trans.Ptr("lucide:shield-check"),
			Order:     trans.Ptr(int32(4)),
			Authority: []string{"sys:platform_admin"},
		},
	},
	{
		Id:        trans.Ptr(uint32(55)),
		ParentId:  trans.Ptr(uint32(50)),
		Type:      permissionV1.Menu_MENU.Enum(),
		Name:      trans.Ptr("PermissionAuditLog"),
		Path:      trans.Ptr("permission-audit-logs"),
		Component: trans.Ptr("app/log/permission_audit_log/index.vue"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Title:     trans.Ptr("menu.log.permissionAuditLog"),
			Icon:      trans.Ptr("lucide:shield-alert"),
			Order:     trans.Ptr(int32(5)),
			Authority: []string{"sys:platform_admin"},
		},
	},

	{
		Id:        trans.Ptr(uint32(60)),
		ParentId:  nil,
		Type:      permissionV1.Menu_CATALOG.Enum(),
		Name:      trans.Ptr("System"),
		Path:      trans.Ptr("/system"),
		Redirect:  trans.Ptr("/system/menus"),
		Component: trans.Ptr("BasicLayout"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Order:     trans.Ptr(int32(2005)),
			Title:     trans.Ptr("menu.system.moduleName"),
			Icon:      trans.Ptr("lucide:settings"),
			KeepAlive: trans.Ptr(true),
			Authority: []string{"sys:platform_admin", "sys:tenant_manager"},
		},
	},
	{
		Id:        trans.Ptr(uint32(61)),
		ParentId:  trans.Ptr(uint32(60)),
		Type:      permissionV1.Menu_MENU.Enum(),
		Name:      trans.Ptr("MenuManagement"),
		Path:      trans.Ptr("menus"),
		Component: trans.Ptr("app/system/menu/index.vue"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Title:     trans.Ptr("menu.system.menu"),
			Icon:      trans.Ptr("lucide:square-menu"),
			Order:     trans.Ptr(int32(1)),
			Authority: []string{"sys:platform_admin"},
		},
	},
	{
		Id:        trans.Ptr(uint32(62)),
		ParentId:  trans.Ptr(uint32(60)),
		Type:      permissionV1.Menu_MENU.Enum(),
		Name:      trans.Ptr("APIManagement"),
		Path:      trans.Ptr("apis"),
		Component: trans.Ptr("app/system/api/index.vue"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Title:     trans.Ptr("menu.system.api"),
			Icon:      trans.Ptr("lucide:route"),
			Order:     trans.Ptr(int32(2)),
			Authority: []string{"sys:platform_admin"},
		},
	},
	{
		Id:        trans.Ptr(uint32(63)),
		ParentId:  trans.Ptr(uint32(60)),
		Type:      permissionV1.Menu_MENU.Enum(),
		Name:      trans.Ptr("DictManagement"),
		Path:      trans.Ptr("dict"),
		Component: trans.Ptr("app/system/dict/index.vue"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Title:     trans.Ptr("menu.system.dict"),
			Icon:      trans.Ptr("lucide:library-big"),
			Order:     trans.Ptr(int32(3)),
			Authority: []string{"sys:platform_admin"},
		},
	},
	{
		Id:        trans.Ptr(uint32(65)),
		ParentId:  trans.Ptr(uint32(60)),
		Type:      permissionV1.Menu_MENU.Enum(),
		Name:      trans.Ptr("TaskManagement"),
		Path:      trans.Ptr("tasks"),
		Component: trans.Ptr("app/system/task/index.vue"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Title:     trans.Ptr("menu.system.task"),
			Icon:      trans.Ptr("lucide:list-todo"),
			Order:     trans.Ptr(int32(5)),
			Authority: []string{"sys:platform_admin", "sys:tenant_manager"},
		},
	},
	{
		Id:        trans.Ptr(uint32(66)),
		ParentId:  trans.Ptr(uint32(60)),
		Type:      permissionV1.Menu_MENU.Enum(),
		Name:      trans.Ptr("LoginPolicyManagement"),
		Path:      trans.Ptr("login-policies"),
		Component: trans.Ptr("app/system/login_policy/index.vue"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Title:     trans.Ptr("menu.system.loginPolicy"),
			Icon:      trans.Ptr("lucide:shield-x"),
			Order:     trans.Ptr(int32(6)),
			Authority: []string{"sys:platform_admin"},
		},
	},
	{
		Id:        trans.Ptr(uint32(67)),
		ParentId:  trans.Ptr(uint32(60)),
		Type:      permissionV1.Menu_MENU.Enum(),
		Name:      trans.Ptr("LanguageManagement"),
		Path:      trans.Ptr("languages"),
		Component: trans.Ptr("app/system/language/index.vue"),
		CreatedAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
		Meta: &permissionV1.MenuMeta{
			Title:     trans.Ptr("menu.system.language"),
			Icon:      trans.Ptr("lucide:globe"),
			Order:     trans.Ptr(int32(7)),
			Authority: []string{"sys:platform_admin"},
		},
	},
}

// init 把运维域种子数据并入默认集,并给平台管理员角色绑定全部运维权限。
// 独立成切片便于维护;init 保证在所有包级变量就绪后执行。
func init() {
	DefaultPermissions = append(DefaultPermissions, DefaultOpsPermissions...)
	DefaultMenus = append(DefaultMenus, defaultOpsMenus()...)

	for _, r := range DefaultRoles {
		if r.Code != nil && *r.Code == PlatformAdminRoleCode {
			existing := r.Permissions
			merged := make([]uint32, 0, len(existing)+len(DefaultOpsPermissions))
			merged = append(merged, existing...)
			for i := range DefaultOpsPermissions {
				if id := DefaultOpsPermissions[i].Id; id != nil {
					merged = append(merged, *id)
				}
			}
			r.Permissions = merged
		}
	}
}

// defaultOpsMenus 运维域菜单:五个一级分组(81-85)+页面(86-101),
// Name/Path/Authority 与前端 ops.tsx 顶级分组静态路由一一对应
// (分组节点无 path,页面挂 /ops/<page>)。
func defaultOpsMenus() []*permissionV1.Menu {
	now := timeutil.TimeToTimestamppb(trans.Ptr(time.Now()))

	group := func(id uint32, name, title, icon string, order int32, authority []string) *permissionV1.Menu {
		return &permissionV1.Menu{
			Id:        trans.Ptr(id),
			ParentId:  nil,
			Type:      permissionV1.Menu_CATALOG.Enum(),
			Name:      trans.Ptr(name),
			Path:      trans.Ptr(""),
			Component: trans.Ptr("BasicLayout"),
			Status:    permissionV1.Menu_ON.Enum(),
			CreatedAt: now,
			Meta: &permissionV1.MenuMeta{
				Title:     trans.Ptr(title),
				Icon:      trans.Ptr(icon),
				Order:     trans.Ptr(order),
				Authority: authority,
			},
		}
	}
	page := func(id uint32, groupId uint32, name, title, icon, authority string) *permissionV1.Menu {
		return &permissionV1.Menu{
			Id:        trans.Ptr(id),
			ParentId:  trans.Ptr(groupId),
			Type:      permissionV1.Menu_MENU.Enum(),
			Name:      trans.Ptr(name),
			Path:      trans.Ptr("/ops/" + name),
			Component: trans.Ptr("BasicLayout"),
			Status:    permissionV1.Menu_ON.Enum(),
			CreatedAt: now,
			Meta: &permissionV1.MenuMeta{
				Title:     trans.Ptr(title),
				Icon:      trans.Ptr(icon),
				Order:     trans.Ptr(int32(id - 85)),
				Authority: []string{authority},
			},
		}
	}
	g81 := group(81, "ops-resource", "routes:opsResource", "lucide:hard-drive", 1,
		[]string{OpsNodeManagePermissionCode, OpsProjectManagePermissionCode})
	g82 := group(82, "ops-cicd", "routes:opsCicd", "lucide:package", 2,
		[]string{OpsBuildManagePermissionCode, OpsDistributePermissionCode})
	g83 := group(83, "ops-auto", "routes:opsAuto", "lucide:bot", 3,
		[]string{OpsScriptManagePermissionCode, OpsNodeManagePermissionCode})
	g84 := group(84, "ops-observe", "routes:opsObserve", "lucide:gauge", 4,
		[]string{SystemPlatformAdminPermissionCode})
	g85 := group(85, "ops-govern", "routes:opsGovern", "lucide:shield-check", 5,
		[]string{SystemPlatformAdminPermissionCode, OpsNodeManagePermissionCode})

	pages := []*permissionV1.Menu{
		page(86, 81, "agent-nodes", "routes:agent-nodes", "lucide:monitor-dot", OpsNodeManagePermissionCode),
		page(87, 81, "terminal", "routes:terminal", "lucide:terminal", OpsNodeManagePermissionCode),
		page(88, 81, "file-manager", "routes:file-manager", "lucide:folder-tree", OpsNodeManagePermissionCode),
		page(89, 81, "projects", "routes:projects", "lucide:folder-open", OpsProjectManagePermissionCode),
		page(90, 82, "repositories", "routes:repositories", "lucide:git-branch", OpsBuildManagePermissionCode),
		page(91, 82, "builds", "routes:builds", "lucide:hammer", OpsBuildManagePermissionCode),
		page(92, 82, "pipelines", "routes:pipelines", "lucide:workflow", OpsBuildManagePermissionCode),
		page(93, 82, "distributions", "routes:distributions", "lucide:send", OpsDistributePermissionCode),
		page(94, 83, "scripts", "routes:scripts", "lucide:file-code", OpsScriptManagePermissionCode),
		page(95, 83, "plugins", "routes:plugins", "lucide:puzzle", OpsNodeManagePermissionCode),
		page(96, 84, "monitors", "routes:monitors", "lucide:activity", SystemPlatformAdminPermissionCode),
		page(97, 84, "alerts", "routes:alerts", "lucide:bell", SystemPlatformAdminPermissionCode),
		page(98, 84, "notification-channels", "routes:notification-channels", "lucide:radio", SystemPlatformAdminPermissionCode),
		page(99, 85, "approvals", "routes:approvals", "lucide:check-circle", SystemPlatformAdminPermissionCode),
		page(100, 85, "command-records", "routes:command-records", "lucide:scroll-text", OpsNodeManagePermissionCode),
		page(101, 85, "api-keys", "routes:api-keys", "lucide:key-round", SystemPlatformAdminPermissionCode),
		page(102, 85, "cloud-accounts", "routes:cloud-accounts", "lucide:cloud", SystemPlatformAdminPermissionCode),
		page(103, 85, "cloud-resources", "routes:cloud-resources", "lucide:server", SystemPlatformAdminPermissionCode),
		page(104, 85, "cloud-costs", "routes:cloud-costs", "lucide:coins", SystemPlatformAdminPermissionCode),
		page(105, 83, "script-batches", "routes:script-batches", "lucide:layers", OpsScriptManagePermissionCode),
		page(106, 83, "backups", "routes:backups", "lucide:archive", OpsScriptManagePermissionCode),
		page(107, 85, "cloud-budgets", "routes:cloud-budgets", "lucide:wallet", SystemPlatformAdminPermissionCode),
		page(108, 85, "certs", "routes:certs", "lucide:file-badge", SystemPlatformAdminPermissionCode),
	}

	return append([]*permissionV1.Menu{g81, g82, g83, g84, g85}, pages...)
}
