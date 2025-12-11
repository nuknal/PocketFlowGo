import { useEffect, useState } from "react";
import { useParams, Link } from "react-router-dom";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { ArrowLeft, Plus } from "lucide-react";
import { api, type FlowVersion } from "@/lib/api";

export default function FlowDetails() {
  const { id } = useParams<{ id: string }>();
  const [versions, setVersions] = useState<FlowVersion[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!id) return;

    const fetchVersions = async () => {
      try {
        const data = await api.getFlowVersions(id);
        setVersions(data || []);
      } catch (error) {
        console.error("Failed to fetch flow versions:", error);
      } finally {
        setLoading(false);
      }
    };

    fetchVersions();
  }, [id]);

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/flows">
          <Button variant="outline" size="icon">
            <ArrowLeft className="h-4 w-4" />
          </Button>
        </Link>
        <div>
          <h1 className="text-3xl font-bold">Flow Details</h1>
          <p className="text-muted-foreground">ID: {id}</p>
        </div>
        <div className="ml-auto">
             <Button>
              <Plus className="mr-2 h-4 w-4" />
              Create Version
            </Button>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Versions</CardTitle>
          <CardDescription>
            History of versions for this flow.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
             <div className="text-center py-4">Loading versions...</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Version</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Definition (Preview)</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {versions.map((version) => (
                  <TableRow key={version.id}>
                    <TableCell className="font-medium">v{version.version}</TableCell>
                    <TableCell>
                      <Badge variant={version.status === "published" ? "default" : "secondary"}>
                        {version.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="max-w-md truncate font-mono text-xs text-muted-foreground">
                      {version.definition_json}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button variant="ghost" size="sm">
                        View Definition
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
                {versions.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={4} className="text-center h-24 text-muted-foreground">
                      No versions found.
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
