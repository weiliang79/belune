import { Plus, Database as DatabaseIcon, AppWindow } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

interface Props {
  onAddApplication: () => void;
  onAddDatabase: () => void;
}

export function ProjectHeader({ onAddApplication, onAddDatabase }: Props) {
  return (
    <div className="flex items-center justify-between">
      <h2 className="text-lg font-semibold">Applications</h2>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button>
              <Plus className="mr-1 h-4 w-4" />
              Add New
            </Button>
          }
        />
        <DropdownMenuContent align="end">
          <DropdownMenuItem onClick={onAddApplication}>
            <AppWindow className="mr-2 h-4 w-4" />
            Application
          </DropdownMenuItem>
          <DropdownMenuItem onClick={onAddDatabase}>
            <DatabaseIcon className="mr-2 h-4 w-4" />
            Database
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
