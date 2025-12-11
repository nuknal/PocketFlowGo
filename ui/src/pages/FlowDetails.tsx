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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { ArrowLeft, Plus } from "lucide-react";
import { api, type FlowVersion } from "@/lib/api";

export default function FlowDetails() {
  const { id } = useParams<{ id: string }>();
  const [versions, setVersions] = useState<FlowVersion[]>([]);
  const [loading, setLoading] = useState(true);
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [newVersion, setNewVersion] = useState("");
  const [newDefinition, setNewDefinition] = useState("");
  const [creating, setCreating] = useState(false);

  const fetchVersions = async () => {
    if (!id) return;
    try {
      const data = await api.getFlowVersions(id);
      setVersions(data || []);
    } catch (error) {
      console.error("Failed to fetch flow versions:", error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchVersions();
  }, [id]);

  const handleCreateVersion = async () => {
    if (!id || !newVersion || !newDefinition) return;
    setCreating(true);
    try {
      await api.createFlowVersion(id, parseInt(newVersion), newDefinition);
      setNewVersion("");
      setNewDefinition("");
      setIsDialogOpen(false);
      fetchVersions();
    } catch (error) {
      console.error("Failed to create flow version:", error);
    } finally {
      setCreating(false);
    }
  };

  const defaultDefinition = JSON.stringify(
    {
      start: "start_node",
      nodes: {
        start_node: {
          kind: "executor",
          service: "transform",
          params: { op: "upper" },
          prep: { input_key: "input" },
          post: { output_key: "output", action_static: "next" },
        },
      },
      edges: [],
    },
    null,
    2
  );

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
             <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
              <DialogTrigger asChild>
                <Button onClick={() => setNewDefinition(defaultDefinition)}>
                  <Plus className="mr-2 h-4 w-4" />
                  Create Version
                </Button>
              </DialogTrigger>
              <DialogContent className="sm:max-w-[600px]">
                <DialogHeader>
                  <DialogTitle>Create Flow Version</DialogTitle>
                  <DialogDescription>
                    Define a new version for this flow.
                  </DialogDescription>
                </DialogHeader>
                <div className="grid gap-4 py-4">
                  <div className="grid grid-cols-4 items-center gap-4">
                    <Label htmlFor="version" className="text-right">
                      Version
                    </Label>
                    <Input
                      id="version"
                      type="number"
                      value={newVersion}
                      onChange={(e) => setNewVersion(e.target.value)}
                      placeholder="e.g. 1"
                      className="col-span-3"
                    />
                  </div>
                  <div className="grid grid-cols-4 items-start gap-4">
                    <Label htmlFor="definition" className="text-right pt-2">
                      Definition (JSON)
                    </Label>
                    <Textarea
                      id="definition"
                      value={newDefinition}
                      onChange={(e) => setNewDefinition(e.target.value)}
                      className="col-span-3 h-[300px] font-mono text-xs"
                    />
                  </div>
                </div>
                <DialogFooter>
                  <Button type="submit" onClick={handleCreateVersion} disabled={creating}>
                    {creating ? "Creating..." : "Create"}
                  </Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
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
                      <Link to={`/flow-versions/${version.id}`}>
                        <Button variant="ghost" size="sm">
                            Details
                        </Button>
                      </Link>
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
