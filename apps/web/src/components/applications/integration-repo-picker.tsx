import { useEffect, useState } from "react";
import { Link } from "@tanstack/react-router";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  useGitIntegrations,
  useIntegrationRepos,
  useIntegrationBranches,
} from "@/lib/hooks/use-git-integrations";

interface Props {
  /** Called whenever a full connection + repo selection is made. */
  onSelect: (v: {
    integrationId: string;
    cloneUrl: string;
    branch: string;
  }) => void;
}

const PROVIDER_LABELS: Record<string, string> = {
  github: "GitHub",
  gitlab: "GitLab",
  bitbucket: "Bitbucket",
  gitea: "Gitea",
};

export function IntegrationRepoPicker({ onSelect }: Props) {
  const { data: integrations } = useGitIntegrations();
  const [integrationId, setIntegrationId] = useState("");
  const [repoFullName, setRepoFullName] = useState("");
  const [branch, setBranch] = useState("");

  const { data: repos } = useIntegrationRepos(integrationId || undefined);
  const { data: branches } = useIntegrationBranches(
    integrationId || undefined,
    repoFullName || undefined,
  );

  const selectedRepo = repos?.find((r) => r.full_name === repoFullName);

  // Emit upward whenever a complete selection exists.
  useEffect(() => {
    if (integrationId && selectedRepo && branch) {
      onSelect({
        integrationId,
        cloneUrl: selectedRepo.clone_url,
        branch,
      });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [integrationId, repoFullName, branch]);

  // Default the branch to the repo's default branch when repo changes.
  useEffect(() => {
    if (selectedRepo) setBranch(selectedRepo.default_branch || "");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [repoFullName]);

  if (!integrations || integrations.length === 0) {
    return (
      <p className="text-muted-foreground text-sm">
        No connected accounts.{" "}
        <Link
          to="/git"
          search={{ tab: undefined, connected: undefined, error: undefined }}
          className="text-foreground font-medium underline underline-offset-2"
        >
          Connect an account
        </Link>{" "}
        under Git to pick a repository here.
      </p>
    );
  }

  return (
    <div className="space-y-3">
      <div className="space-y-2">
        <Label>Connection</Label>
        <Select
          value={integrationId}
          onValueChange={(v) => {
            setIntegrationId(v ?? "");
            setRepoFullName("");
            setBranch("");
          }}
        >
          <SelectTrigger>
            <SelectValue placeholder="Select a connected account" />
          </SelectTrigger>
          <SelectContent>
            {integrations.map((i) => (
              <SelectItem key={i.id} value={i.id}>
                {PROVIDER_LABELS[i.provider] ?? i.provider} · {i.account_login}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {integrationId && (
        <div className="space-y-2">
          <Label>Repository</Label>
          <Select
            value={repoFullName}
            onValueChange={(v) => setRepoFullName(v ?? "")}
          >
            <SelectTrigger>
              <SelectValue placeholder="Select a repository" />
            </SelectTrigger>
            <SelectContent>
              {repos?.map((r) => (
                <SelectItem key={r.full_name} value={r.full_name}>
                  {r.full_name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}

      {repoFullName && (
        <div className="space-y-2">
          <Label>Branch</Label>
          <Select value={branch} onValueChange={(v) => setBranch(v ?? "")}>
            <SelectTrigger>
              <SelectValue placeholder="Select a branch" />
            </SelectTrigger>
            <SelectContent>
              {branches?.map((b) => (
                <SelectItem key={b.name} value={b.name}>
                  {b.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}
    </div>
  );
}
