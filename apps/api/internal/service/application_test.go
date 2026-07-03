package service_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/service"
	"github.com/ungweiliang/selfhost-paas/internal/testutil"
)

func TestApplicationService_Create_FinalizesSlugAndWebhookSecret(t *testing.T) {
	t.Cleanup(func() { truncate(t) })

	_, project := seedUserAndProject(t)
	rt := &testutil.MockContainerRuntime{}
	svc := service.NewApplicationService(testPool, testQueries, rt, testKeyring, t.TempDir())

	app, err := svc.Create(context.Background(), service.CreateApplicationParams{
		ProjectID:   project.ID,
		ProjectSlug: project.Slug,
		Name:        "My Web App",
		BaseSlug:    "web",
		Type:        "image",
		SourceImage: "nginx:alpine",
		BuildType:   "image",
	})
	require.NoError(t, err)

	// Final slug must be {projectSlug}-{baseSlug}-{id[:8]} so it is unique
	// across re-creations and stable for naming.ContainerName.
	idStr := uuidString(app.ID)
	wantPrefix := project.Slug + "-web-" + idStr[:8]
	assert.Equal(t, wantPrefix, app.Slug, "Create should finalize slug with id prefix")

	// Persisted row should match the returned struct (no double-write surprises).
	persisted, err := testQueries.GetApplication(context.Background(), app.ID)
	require.NoError(t, err)
	assert.Equal(t, wantPrefix, persisted.Slug)

	// A non-empty webhook secret is required for git push verification — the
	// service must never persist an application without one, even when the
	// caller didn't ask for git auth.
	assert.True(t, persisted.WebhookSecret.Valid)
	assert.NotEmpty(t, persisted.WebhookSecret.String)
}

func TestApplicationService_Create_EncryptsGitToken(t *testing.T) {
	t.Cleanup(func() { truncate(t) })

	_, project := seedUserAndProject(t)
	rt := &testutil.MockContainerRuntime{}
	svc := service.NewApplicationService(testPool, testQueries, rt, testKeyring, t.TempDir())

	const plaintextPAT = "ghp_supersecrettokenvalue123"
	app, err := svc.Create(context.Background(), service.CreateApplicationParams{
		ProjectID:   project.ID,
		ProjectSlug: project.Slug,
		Name:        "App With PAT",
		BaseSlug:    "patapp",
		Type:        "git",
		SourceRepo:  "https://github.com/example/repo",
		BuildType:   "dockerfile",
		GitToken:    plaintextPAT,
	})
	require.NoError(t, err)

	persisted, err := testQueries.GetApplication(context.Background(), app.ID)
	require.NoError(t, err)

	// Bytes on disk must not contain the plaintext PAT — that's the entire
	// point of the keyring. Also assert decryption round-trips so we know we
	// stored ciphertext, not garbage.
	require.NotEmpty(t, persisted.GitCredentialsEncrypted)
	assert.False(t, bytes.Contains(persisted.GitCredentialsEncrypted, []byte(plaintextPAT)),
		"raw plaintext PAT must never appear in stored bytes")

	decrypted, err := testKeyring.Decrypt(persisted.GitCredentialsEncrypted)
	require.NoError(t, err)
	assert.Equal(t, plaintextPAT, string(decrypted),
		"stored ciphertext should decrypt back to original PAT")
}

func TestApplicationService_Delete_StopsAndRemovesAllContainerNames(t *testing.T) {
	t.Cleanup(func() { truncate(t) })

	_, project := seedUserAndProject(t)
	rt := &testutil.MockContainerRuntime{}
	svc := service.NewApplicationService(testPool, testQueries, rt, testKeyring, t.TempDir())

	app, err := svc.Create(context.Background(), service.CreateApplicationParams{
		ProjectID:   project.ID,
		ProjectSlug: project.Slug,
		Name:        "Doomed",
		BaseSlug:    "doom",
		Type:        "image",
		SourceImage: "nginx:alpine",
		BuildType:   "image",
	})
	require.NoError(t, err)

	idStr := uuidString(app.ID)
	currentName := naming.ContainerName(project.Slug, app.Slug, idStr)
	intermediateName := naming.IntermediateContainerName(project.Slug, idStr)
	oldName := naming.OldContainerName(idStr)

	require.NoError(t, svc.Delete(context.Background(), app.ID, project.Slug, app.Slug))

	// Delete must reach for all three legacy/current naming variants — the
	// migration history left previous containers under different names, and a
	// missed cleanup leaks resources permanently. Containment check, not
	// equality, because Delete also calls RemoveVolume on cache volumes via
	// other runtime methods unrelated to this assertion.
	for _, name := range []string{currentName, intermediateName, oldName} {
		assert.Contains(t, rt.StopCalls, name,
			"Delete should attempt to stop container variant %q", name)
		assert.Contains(t, rt.RemoveCalls, name,
			"Delete should attempt to remove container variant %q", name)
	}

	// DB row must be gone (this is how we know the cascade fires for
	// deployments / env vars / domains in production).
	_, err = testQueries.GetApplication(context.Background(), app.ID)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "no rows"),
		"GetApplication should return ErrNoRows after Delete; got %v", err)
}
