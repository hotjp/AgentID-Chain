# gomock 使用说明

## 生成 mock

```bash
# 生成全部
./internal/testutil/gomock/gen.sh

# 生成指定包
./internal/testutil/gomock/gen.sh internal/storage
```

## 命名约定

- 输出文件：`{interface_name_snake}_mock.go`
- 包名：`mock_{path}`（用 `_` 替换 `/`）
- 例如：`internal/storage.UserRepo` → `internal/testutil/gomock/storage/user_repo_mock.go`（包 `mock_storage`）

## 依赖

```
go install go.uber.org/mock/mockgen@latest
```

## 单元测试中使用

```go
import (
    "testing"
    "github.com/golang/mock/gomock"

    mockstorage "github.com/agentid-chain/agentid-chain/internal/testutil/gomock/storage"
)

func TestSomething(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    repo := mockstorage.NewMockUserRepo(ctrl)
    repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, nil)

    // ... 你的测试逻辑
}
```
