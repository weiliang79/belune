import { useCallback } from "react";
import { useForm } from "@tanstack/react-form";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useUpdateWebhook } from "@/lib/hooks/use-applications";
import { CopyButton } from "@/lib/components/copy-button";
import type { Application } from "@/lib/types";

interface Props {
  projectId: string;
  applicationId: string;
  application: Application;
}

export function WebhookSection({
  projectId,
  applicationId,
  application,
}: Props) {
  const updateWebhook = useUpdateWebhook(projectId, applicationId);

  const webhookForm = useForm({
    defaultValues: {
      webhook_secret: application.webhook_secret ?? "",
      auto_deploy_branch: application.auto_deploy_branch ?? "main",
    },
    onSubmit: async ({ value }) => {
      toast.promise(
        updateWebhook.mutateAsync({
          webhook_secret: value.webhook_secret,
          auto_deploy_branch: value.auto_deploy_branch || "main",
        }),
        {
          loading: "Saving...",
          success: "Webhook settings saved",
          error: (err) => err.message,
        },
      );
    },
  });

  const generateSecret = useCallback(() => {
    const bytes = new Uint8Array(32);
    crypto.getRandomValues(bytes);
    const secret = Array.from(bytes, (b) =>
      b.toString(16).padStart(2, "0"),
    ).join("");
    webhookForm.setFieldValue("webhook_secret", secret);
  }, [webhookForm]);

  return (
    <Card>
      <CardHeader>
        <CardTitle>Webhook (Auto-Deploy)</CardTitle>
      </CardHeader>
      <CardContent>
        <p className="text-muted-foreground text-sm mb-4">
          Configure a webhook secret to enable push-to-deploy. Add this URL as a
          webhook in your GitHub or GitLab repository:
        </p>
        <div className="bg-muted flex items-center justify-between rounded-md px-3 py-2 mb-4">
          <code className="text-sm font-mono break-all">
            {window.location.origin}/api/webhooks/push
          </code>
          <CopyButton value={`${window.location.origin}/api/webhooks/push`} />
        </div>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            e.stopPropagation();
            webhookForm.handleSubmit();
          }}
          className="space-y-4"
        >
          <webhookForm.Field
            name="webhook_secret"
            children={(field) => (
              <div className="space-y-2">
                <Label>Webhook Secret</Label>
                <div className="flex gap-2">
                  <Input
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                    placeholder="Enter or generate a secret"
                    className="font-mono"
                  />
                  <Button
                    type="button"
                    variant="outline"
                    onClick={generateSecret}
                  >
                    Generate
                  </Button>
                </div>
                <p className="text-muted-foreground text-xs">
                  Use this secret when configuring the webhook in your git
                  provider.
                </p>
              </div>
            )}
          />
          <webhookForm.Field
            name="auto_deploy_branch"
            children={(field) => (
              <div className="space-y-2">
                <Label>Auto-Deploy Branch</Label>
                <Input
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.target.value)}
                  placeholder="main"
                />
                <p className="text-muted-foreground text-xs">
                  Only pushes to this branch will trigger a deploy.
                </p>
              </div>
            )}
          />
          <webhookForm.Subscribe
            selector={(s) => s.isSubmitting}
            children={(isSubmitting) => (
              <Button type="submit" disabled={isSubmitting}>
                {isSubmitting ? "Saving..." : "Save Webhook Settings"}
              </Button>
            )}
          />
        </form>
      </CardContent>
    </Card>
  );
}
