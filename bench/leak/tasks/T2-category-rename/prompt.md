Add the backend for renaming a category.

The frontend's rename action already calls `PATCH /api/projects/{projectID}/categories/{id}` (`updateCategory` in `frontend/src/lib/api/categories.ts`), but the backend doesn't serve that route, so renaming fails.

- Request body `{"name": "..."}`. Any member of the project may rename a category.
- Validate the name the way creating a category does: 400 `API_NAME_REQUIRED` and 400 `API_NAME_TOO_LONG`. If the project already has a category with that name, 409 `CATEGORY_DUPLICATE_NAME`.
- An unknown category is 404 `CATEGORY_NOT_FOUND`.
- On success, respond 200 with the renamed category and publish the `category.renamed` realtime event (`realtime.CategoryRenamedPayload`).

Nobody is available to answer questions in this session. Where a workflow asks you to confirm something with the user, decide yourself, say what you decided in your final message, and carry on until the change is implemented.

Follow the repository's conventions. Add or update tests. Commit your work on the current branch when done.
