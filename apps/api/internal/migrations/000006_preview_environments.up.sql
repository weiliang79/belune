-- Preview environments: an application may be a "preview" child of another
-- application. The parent holds the branch-matching pattern and domain
-- template; children are materialized on webhook push when the pushed branch
-- matches the pattern. Children reuse all existing deploy/container/route
-- machinery; they are distinguished only by parent_application_id + branch.

ALTER TABLE applications
    ADD COLUMN parent_application_id UUID REFERENCES applications(id) ON DELETE CASCADE,
    ADD COLUMN branch VARCHAR(255),
    ADD COLUMN preview_branch_pattern VARCHAR(255),
    ADD COLUMN preview_domain_template VARCHAR(255),
    ADD COLUMN last_activity_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX idx_applications_parent_id ON applications(parent_application_id);
CREATE UNIQUE INDEX idx_applications_parent_branch
    ON applications(parent_application_id, branch)
    WHERE parent_application_id IS NOT NULL;
