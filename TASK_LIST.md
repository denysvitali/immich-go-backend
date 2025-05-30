# Immich Go Backend - Protobuf Implementation Task List

## Overview
Based on the analysis of the Immich OpenAPI specification (v1.134.0), this document outlines the tasks needed to implement missing protobuf definitions and ensure complete API coverage.

## Current Status
- **Total API endpoints**: 193 endpoints across 28 categories
- **Existing protobuf files**: 15 files covering 15 categories
- **Missing protobuf files**: 13 files needed for complete coverage

## Tasks to Complete

### 1. Create Missing Protobuf Files

#### 1.1 download.proto (2 endpoints)
- `POST /download/archive` → downloadArchive
- `POST /download/info` → getDownloadInfo

#### 1.2 faces.proto (4 endpoints)
- `GET /faces` → getFaces
- `POST /faces` → createFace
- `DELETE /faces/{id}` → deleteFace
- `PUT /faces/{id}` → reassignFacesById

#### 1.3 libraries.proto (8 endpoints)
- `GET /libraries` → getAllLibraries
- `POST /libraries` → createLibrary
- `DELETE /libraries/{id}` → deleteLibrary
- `GET /libraries/{id}` → getLibrary
- `PUT /libraries/{id}` → updateLibrary
- `POST /libraries/{id}/scan` → scanLibrary
- `GET /libraries/{id}/statistics` → getLibraryStatistics
- `POST /libraries/{id}/validate` → validate

#### 1.4 partners.proto (4 endpoints)
- `GET /partners` → getPartners
- `DELETE /partners/{id}` → removePartner
- `POST /partners/{id}` → createPartner
- `PUT /partners/{id}` → updatePartner

#### 1.5 people.proto (9 endpoints)
- `GET /people` → getAllPeople
- `POST /people` → createPerson
- `PUT /people` → updatePeople
- `GET /people/{id}` → getPerson
- `PUT /people/{id}` → updatePerson
- `POST /people/{id}/merge` → mergePerson
- `PUT /people/{id}/reassign` → reassignFaces
- `GET /people/{id}/statistics` → getPersonStatistics
- `GET /people/{id}/thumbnail` → getPersonThumbnail

#### 1.6 sessions.proto (5 endpoints)
- `DELETE /sessions` → deleteAllSessions
- `GET /sessions` → getSessions
- `POST /sessions` → createSession
- `DELETE /sessions/{id}` → deleteSession
- `POST /sessions/{id}/lock` → lockSession

#### 1.7 shared_links.proto (8 endpoints)
- `GET /shared-links` → getAllSharedLinks
- `POST /shared-links` → createSharedLink
- `GET /shared-links/me` → getMySharedLink
- `DELETE /shared-links/{id}` → removeSharedLink
- `GET /shared-links/{id}` → getSharedLinkById
- `PATCH /shared-links/{id}` → updateSharedLink
- `DELETE /shared-links/{id}/assets` → removeSharedLinkAssets
- `PUT /shared-links/{id}/assets` → addSharedLinkAssets

#### 1.8 stacks.proto (6 endpoints)
- `DELETE /stacks` → deleteStacks
- `GET /stacks` → searchStacks
- `POST /stacks` → createStack
- `DELETE /stacks/{id}` → deleteStack
- `GET /stacks/{id}` → getStack
- `PUT /stacks/{id}` → updateStack

#### 1.9 sync.proto (6 endpoints)
- `DELETE /sync/ack` → deleteSyncAck
- `GET /sync/ack` → getSyncAck
- `POST /sync/ack` → sendSyncAck
- `POST /sync/delta-sync` → getDeltaSync
- `POST /sync/full-sync` → getFullSyncForUser
- `POST /sync/stream` → getSyncStream

#### 1.10 tags.proto (9 endpoints)
- `GET /tags` → getAllTags
- `POST /tags` → createTag
- `PUT /tags` → upsertTags
- `PUT /tags/assets` → bulkTagAssets
- `DELETE /tags/{id}` → deleteTag
- `GET /tags/{id}` → getTagById
- `PUT /tags/{id}` → updateTag
- `DELETE /tags/{id}/assets` → untagAssets
- `PUT /tags/{id}/assets` → tagAssets

#### 1.11 trash.proto (3 endpoints)
- `POST /trash/empty` → emptyTrash
- `POST /trash/restore` → restoreTrash
- `POST /trash/restore/assets` → restoreAssets

#### 1.12 view.proto (2 endpoints)
- `GET /view/folder` → getAssetsByOriginalPath
- `GET /view/folder/unique-paths` → getUniqueOriginalPaths

#### 1.13 duplicates.proto (1 endpoint)
- `GET /duplicates` → getAssetDuplicates

### 2. Handle Special Categories

#### 2.1 Create additional protobuf files for "other" category endpoints:
- **map.proto** for map-related endpoints:
  - `GET /map/markers` → getMapMarkers
  - `GET /map/reverse-geocode` → reverseGeocode

- **oauth.proto** for OAuth endpoints:
  - `POST /oauth/authorize` → startOAuth
  - `POST /oauth/callback` → finishOAuth
  - `POST /oauth/link` → linkOAuthAccount
  - `GET /oauth/mobile-redirect` → redirectOAuthToMobile
  - `POST /oauth/unlink` → unlinkOAuthAccount

- **system_metadata.proto** for system metadata endpoints:
  - `GET /system-metadata/admin-onboarding` → getAdminOnboarding
  - `POST /system-metadata/admin-onboarding` → updateAdminOnboarding
  - `GET /system-metadata/reverse-geocoding-state` → getReverseGeocodingState
  - `GET /system-metadata/version-check-state` → getVersionCheckState

### 3. Verify Existing Protobuf Files
- Review existing protobuf files to ensure they include all endpoints from their respective categories
- Check for any missing methods or outdated definitions
- Ensure consistency with OpenAPI spec data models

### 4. Generate and Test
- Use `buf generate` to generate Go code from all protobuf files
- Ensure all generated code compiles without errors
- Add TODO comments for implementation in service handlers

### 5. Update Build Configuration
- Verify `buf.gen.yaml` and `buf.yaml` are properly configured
- Ensure all new protobuf files are included in the build process

## Implementation Notes
- All protobuf files should follow the existing pattern with `immich.v1` package
- Use appropriate HTTP annotations for REST API mapping
- Include proper Go package options
- Add TODO comments for database integration and business logic
- Follow existing code structure and naming conventions

## Success Criteria
- All 193 API endpoints have corresponding protobuf definitions
- All protobuf files compile successfully with `buf generate`
- Generated Go code compiles without errors
- Code is ready for implementation (with TODOs for business logic)