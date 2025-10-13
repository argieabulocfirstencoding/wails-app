import { Info } from "lucide-react";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";

interface Props {
  title?: string;
  children: React.ReactNode;
}

export function InfoAlert({ title = "Info", children }: Props) {
  return (
    <Alert className="bg-blue-50 border-blue-100 text-blue-800">
      <div className="flex items-start gap-3">
        <Info className="w-5 h-5 text-blue-600 mt-0.5" />
        <div className="flex-1">
          <AlertTitle className="font-semibold">{title}</AlertTitle>
          <AlertDescription className="text-sm">{children}</AlertDescription>
        </div>
      </div>
    </Alert>
  );
}
