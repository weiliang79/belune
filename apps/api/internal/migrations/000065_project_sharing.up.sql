-- Let a project be shared with every Member in the install.
--
-- A project has exactly one owner today, so two Members can never collaborate
-- on the same project — any team sharing work has to escalate everyone to
-- Admin just to get access. v1 is a flag, not a membership table: a shared
-- project is visible and usable by every Member, the owner keeps destructive
-- rights (delete, transfer, change sharing). Real project_members can follow
-- later as an additive change — `shared` would simply become "everyone is a
-- member" for existing rows.
ALTER TABLE projects ADD COLUMN shared BOOLEAN NOT NULL DEFAULT false;
