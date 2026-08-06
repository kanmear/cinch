I've now read all test files across both handlers and models. Let me compile the complete audit report.

## DeltaDocs Rule-Coverage Audit Report

### Summary

| Domain | Total Rules | API-Covered | Model-Covered | N/A | Uncovered | Coverage |
|--------|-------------|-------------|---------------|-----|-----------|----------|
| **Projects (PROJ)** | 9 | 9 | 9 | 0 | 0 | **100%** |
| **Categories (CAT)** | 6 | 4 | 4 | 2 | 0 | **100%** (tested) |
| **Roles (ROLE)** | 3 | 3 | 3 | 0 | 0 | **100%** |
| **Tabs (TAB)** | 5 | 3 | 4 | 0 | 1 | **80%** |
| **Tabs Permissions (TABPERM)** | 10 | 10 | 0 | 0 | 0 | **100%** |
| **Members (MEMB)** | 8 | 5 | 5 | 1 | 0 | **100%** (tested) |
| **Permissions (PERM)** | 2 | 2 | 2 | 0 | 0 | **100%** |
| **Access (ACCESS)** | 14 | 14 | 0 | 0 | 0 | **100%** |
| **Files (FILE)** | 5 | 5 | 5 | 0 | 0 | **100%** |
| **Signatures (SIG)** | 7 | 0 | 7 | 0 | 0 | **100%** |
| **TOTAL** | **69** | **55** | **39** | **3** | **1** | **98.6%** |

---

### Per-Domain Detail

#### Projects (PROJ) — 9/9 covered, 0 uncovered

| Rule | Classification | Test File | Test Function |
|------|---------------|-----------|---------------|
| PROJ-001 | API | `project_test.go` | `TestCreateProject_OwnerIsFirstMember` |
| PROJ-002 | API + Model | `project_test.go` | `TestCreateProject_Roles` + `TestCreateProject` |
| PROJ-003 | API + Model | `project_test.go` | `TestCreateProject_OwnerPermission` + `user_test.go` `TestDeleteUser_CascadesProjects` |
| PROJ-004 | Model | `project_test.go` | `TestCreateProject` |
| PROJ-005 | API + Model | `project_test.go` | `TestUpdateProject_OnlyOwnerCanRename` + `TestTransferProjectOwnership` |
| PROJ-006 | API | `project_test.go` | `TestCreateProject_DuplicateName` |
| PROJ-007 | API + Model | `project_test.go` | `TestDeleteProject_OnlyOwnerCanDelete` + `TestDeleteProject` |
| PROJ-008 | API + Model | `project_test.go` | `TestDeleteProject_NotOwnerCannotDelete` + `TestDeleteProject` |
| PROJ-009 | API + Model | `project_test.go` | `TestUpdateProject_NotOwnerCannotRename` + `user_test.go` `TestDeleteUser_CascadesProjects` |

#### Categories (CAT) — 4/6 tested, 2 N/A, 0 uncovered

| Rule | Classification | Test File | Test Function |
|------|---------------|-----------|---------------|
| CAT-001 | API + Model | `category_test.go` | `TestCreateCategory_OnlyOwnerCanAdd` + `TestCreateCategory` |
| CAT-002 | API | `category_test.go` | `TestCreateCategory_DuplicateName` |
| CAT-003 | **N/A** | — | — |
| CAT-004 | API + Model | `category_test.go` | `TestDeleteCategory_OnlyOwnerCanDelete` + `TestDeleteCategory_CascadesTabs` |
| CAT-005 | **N/A** | — | — |

#### Roles (ROLE) — 3/3 covered, 0 uncovered

| Rule | Classification | Test File | Test Function |
|------|---------------|-----------|---------------|
| ROLE-001 | API + Model | `role_test.go` | `TestCreateRole_OnlyOwnerCanAdd` + `TestCreateRole_DuplicateName` |
| ROLE-002 | API + Model | `role_test.go` | `TestCreateRole_DuplicateName` + `project_test.go` `TestCreateProject` |
| ROLE-003 | API + Model | `role_test.go` | `TestCreateRole_OnlyOwnerCanAdd` + `TestCreateRole` |

#### Tabs (TAB) — 4/5 covered, 1 **UNCOVERED**

| Rule | Classification | Test File | Test Function |
|------|---------------|-----------|---------------|
| TAB-001 | API | `tab_test.go` | `TestCreateTab_OnlyOwnerCanAdd` |
| TAB-002 | API + Model | `tab_test.go` | `TestCreateTab_DuplicateCategory` + `TestCreateTab_DuplicateCategoryPerPage` |
| TAB-003 | API + Model | `tab_test.go` | `TestCreateTab_OnlyOwnerCanAdd` (duplicate marker) + `TestUpdateTab_CreatesHistoryEntry` |
| **TAB-004** | **Model** | **---** | **NO MARKER FOUND** |
| TAB-005 | Model | `tab_test.go` | `TestGetTabsByPage_SignedFalseAfterSubsequentEdit` |

**TAB-004 gap**: "A tab is 'signed' when its content hash matches the signed baseline." The test `TestGetTabsByPage_SignedTrueAfterAllAccept` in `tab_test.go` (model) validates this behavior but has **no `cinch:rule` marker**.

#### Tabs Permissions (TABPERM) — 10/10 covered, 0 uncovered

| Rule | Classification | Test File | Test Function |
|------|---------------|-----------|---------------|
| TABPERM-001 | API | `tab_permission_test.go` | `TestListTabs_ViewerCanList` |
| TABPERM-002 | API | `tab_permission_test.go` | `TestListTabs_NonMemberCannotList` |
| TABPERM-003 | API | `tab_permission_test.go` | `TestGetTab_ViewerCanView` |
| TABPERM-004 | API | `tab_permission_test.go` | `TestGetTab_NonMemberCannotView` |
| TABPERM-005 | API | `tab_permission_test.go` | `TestCreateTab_ViewerCannotCreate` |
| TABPERM-006 | API | `tab_permission_test.go` | `TestCreateTab_NonMemberCannotCreate` |
| TABPERM-007 | API | `tab_permission_test.go` | `TestUpdateTab_ViewerCannotUpdate` |
| TABPERM-008 | API | `tab_permission_test.go` | `TestUpdateTab_NonMemberCannotUpdate` |
| TABPERM-009 | API | `tab_permission_test.go` | `TestDeleteTab_ViewerCannotDelete` |
| TABPERM-010 | API | `tab_permission_test.go` | `TestDeleteTab_NonMemberCannotDelete` |

#### Members (MEMB) — 7/8 tested, 1 N/A, 0 uncovered

| Rule | Classification | Test File | Test Function |
|------|---------------|-----------|---------------|
| MEMB-001 | API + Model | `member_test.go` | `TestAddMember_OnlyOwnerCanAdd` + `TestAddMember_DuplicateProjectUser` |
| MEMB-002 | API + Model | `member_test.go` | `TestAddMember_DuplicateMember` + `TestAddMember` |
| MEMB-003 | API | `member_test.go` | `TestRemoveMember_OnlyOwnerCanRemove` |
| MEMB-004 | API | `member_test.go` | `TestUpdateMemberRole_OnlyOwnerCanUpdate` |
| MEMB-005 | API + Model | `member_test.go` | `TestAddMember_OnlyOwnerCanAdd` (duplicate marker) + `project_test.go` `TestCreateProject` |
| MEMB-006 | **N/A** | — | — |
| MEMB-007 | Model | `member_test.go` | `TestRemoveMember` |
| MEMB-008 | Model | `user_test.go` | `TestDeleteUser_RemovesMemberships` |

#### Permissions (PERM) — 2/2 covered, 0 uncovered

| Rule | Classification | Test File | Test Function |
|------|---------------|-----------|---------------|
| PERM-001 | API + Model | `permission_test.go` | `TestSetPermission_OnlyOwnerCanSet` + `role_category_permission_test.go` `TestSetPermission_InvalidLevel` |
| PERM-002 | API + Model | `permission_test.go` | `TestSetPermission_NonMemberCannotSet` + `role_category_permission_test.go` `TestSetPermission_Upsert` |

#### Access (ACCESS) — 14/14 covered, 0 uncovered

| Rule | Classification | Test File | Test Function |
|------|---------------|-----------|---------------|
| ACCESS-001 | API | `access_test.go` | `TestCanAccess_ViewerCanAccess` |
| ACCESS-002 | API | `access_test.go` | `TestCanAccess_NonMemberCannotAccess` |
| ACCESS-003 | API | `access_test.go` | `TestCanAccess_NoPermissionCannotAccess` |
| ACCESS-004 | API | `access_test.go` | `TestCanAccess_EditorCanAccess` |
| ACCESS-005 | API | `access_test.go` | `TestCanAccess_OwnerCanAccess` |
| ACCESS-006 | API | `access_test.go` | `TestCanAccess_NoPermissionCannotAccess` (duplicate marker) |
| ACCESS-007 | API | `access_test.go` | `TestCanAccess_NoPermissionCannotAccess` (duplicate marker) |
| ACCESS-008 | API | `access_checks_test.go` | `TestCheckAccess_ViewerCanAccess` |
| ACCESS-009 | API | `access_checks_test.go` | `TestCheckAccess_NonMemberCannotAccess` |
| ACCESS-010 | API | `access_checks_test.go` | `TestCheckAccess_NoPermissionCannotAccess` |
| ACCESS-011 | API | `access_checks_test.go` | `TestCheckAccess_EditorCanAccess` |
| ACCESS-012 | API | `access_checks_test.go` | `TestCheckAccess_OwnerCanAccess` |
| ACCESS-013 | API | `access_checks_test.go` | `TestCheckAccess_NoPermissionCannotAccess` (duplicate marker) |
| ACCESS-014 | API | `access_checks_test.go` | `TestCheckAccess_NoPermissionCannotAccess` (duplicate marker) |

#### Files (FILE) — 5/5 covered, 0 uncovered

| Rule | Classification | Test File | Test Function |
|------|---------------|-----------|---------------|
| FILE-001 | API + Model | `file_test.go` | `TestCreateFile_OnlyOwnerCanAdd` + `TestCreateFile_InvalidType` |
| FILE-002 | API + Model | `file_test.go` | `TestCreateFile_DuplicateName` + `TestCreateFile_WithParent` |
| FILE-003 | API + Model | `file_test.go` | `TestCreateFile_OnlyOwnerCanAdd` (duplicate marker) + `TestCreateFile_Root` |
| FILE-004 | API + Model | `file_test.go` | `TestDeleteFile_OnlyOwnerCanDelete` + `TestDeleteFile_CascadesChildren` |
| FILE-005 | API + Model | `file_test.go` | `TestDeleteFile_OnlyOwnerCanDelete` (duplicate marker) + `TestMoveFile_CompactsOldSiblings` |

#### Signatures (SIG) — 7/7 covered, 0 uncovered

| Rule | Classification | Test File | Test Function |
|------|---------------|-----------|---------------|
| SIG-004 | Model | `tab_test.go` | `TestCreateSignatureRequest_SnapshotsContentAndPinsHistory` |
| SIG-009 | Model | `tab_test.go` | `TestOrdered_BothAccept_SignsBaseline` |
| SIG-010 | Model | `tab_test.go` | `TestOrdered_DeclineResolvesImmediatelyAndStampsDownstream` |
| SIG-012 | Model | `tab_test.go` | `TestUpdateTab_SucceedsWithPendingSignature` |
| SIG-014 | Model | `tab_test.go` | `TestGetTab_SignedByCarriesBaselineRoster` |
| SIG-018 | Model | `tab_test.go` | `TestCreateSignatureRequest_SynthesizesHistoryWhenNoneExists` |
| SIG-019 | Model | `tab_test.go` | `TestUnordered_FinalAcceptAfterDeclineResolvesDeclinedWithoutBaseline` |

---

### Duplicate `cinch:rule` Markers (informational)

The following markers appear on tests where the test name/functionality doesn't match the rule, or the marker appears in multiple places:

| Rule | File | Test Function | Issue |
|------|------|---------------|-------|
| CAT-003 | `category_test.go` | `TestCreateCategory_OnlyOwnerCanAdd` | Duplicate marker; rule is N/A |
| CAT-005 | `category_test.go` | `TestDeleteCategory_OnlyOwnerCanDelete` | Duplicate marker; rule is N/A |
| MEMB-005 | `member_test.go` | `TestAddMember_OnlyOwnerCanAdd` | Duplicate marker; covered in `project_test.go` `TestCreateProject` |
| PROJ-003 | `user_test.go` | `TestDeleteUser_CascadesProjects` | Also marked on `project_test.go` `TestCreateProject_OwnerPermission` |
| PROJ-009 | `user_test.go` | `TestDeleteUser_CascadesProjects` | Also marked on `project_test.go` `TestUpdateProject_NotOwnerCannotRename` |
| ROLE-001 | `role_test.go` (model) | `TestCreateRole_DuplicateName` | Duplicate marker; also on `TestCreateRole_OnlyOwnerCanAdd` |
| ROLE-002 | `project_test.go` | `TestCreateProject` | Also marked on `role_test.go` `TestCreateRole_DuplicateName` |
| ROLE-003 | `role_test.go` (model) | `TestCreateRole` | Also marked on `role_test.go` `TestCreateRole_OnlyOwnerCanAdd` |
| TAB-002 | `tab_test.go` (model) | `TestCreateTab_DuplicateCategoryPerPage` | Also marked on `tab_test.go` `TestCreateTab_DuplicateCategory` |
| TAB-003 | `tab_test.go` (handler) | `TestCreateTab_OnlyOwnerCanAdd` | Wrong test; actual coverage is `TestUpdateTab_CreatesHistoryEntry` |
| FILE-003 | `file_test.go` (handler) | `TestCreateFile_OnlyOwnerCanAdd` | Duplicate marker; actual coverage is `TestCreateFile_Root` |
| FILE-005 | `file_test.go` (handler) | `TestDeleteFile_OnlyOwnerCanDelete` | Duplicate marker; actual coverage is `TestMoveFile_CompactsOldSiblings` |

---

### Action Required

**TAB-004** — Add `// cinch:rule TAB-004` to `backend/tests/models/tab_test.go` on the `TestGetTabsByPage_SignedTrueAfterAllAccept` function (line ~920). The test already validates the rule but is missing the marker.
