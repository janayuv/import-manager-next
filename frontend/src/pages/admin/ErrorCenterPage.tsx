import * as React from 'react';
import { toast } from 'sonner';
import { AlertCircle, Bug, CheckCircle2, Plus, RefreshCw, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Textarea } from '@/components/ui/textarea';
import { workflowService, type BugNote, type ErrorMemoryRecord } from '@/services/workflow-service';

export function ErrorCenterPage() {
  const [errors, setErrors] = React.useState<ErrorMemoryRecord[]>([]);
  const [bugNotes, setBugNotes] = React.useState<BugNote[]>([]);
  const [statusFilter, setStatusFilter] = React.useState('all');

  const [bugTitle, setBugTitle] = React.useState('');
  const [bugDesc, setBugDesc] = React.useState('');

  const refresh = React.useCallback(async () => {
    try {
      const [errRows, notes] = await Promise.all([
        workflowService.listErrors(statusFilter, 'all'),
        workflowService.listBugNotes(),
      ]);
      setErrors(errRows);
      setBugNotes(notes);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to load error center data');
    }
  }, [statusFilter]);

  React.useEffect(() => {
    void refresh();
  }, [refresh]);

  const handleUpdateErrorStatus = async (id: string, status: string) => {
    try {
      await workflowService.updateErrorStatus(id, status, '');
      toast.success(`Error marked ${status}`);
      await refresh();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Status update failed');
    }
  };

  const handleCreateBugNote = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await workflowService.saveBugNote({
        title: bugTitle,
        description: bugDesc,
        status: 'OPEN',
      });
      setBugTitle('');
      setBugDesc('');
      toast.success('Bug note saved');
      await refresh();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to save bug note');
    }
  };

  const handleToggleBugNote = async (note: BugNote) => {
    try {
      await workflowService.saveBugNote({
        id: note.id,
        title: note.title,
        description: note.description,
        status: note.status === 'OPEN' ? 'SOLVED' : 'OPEN',
      });
      toast.success(`Bug note marked ${note.status === 'OPEN' ? 'SOLVED' : 'OPEN'}`);
      await refresh();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to update bug note');
    }
  };

  const handleDeleteBugNote = async (id: string) => {
    try {
      await workflowService.deleteBugNote(id);
      toast.success('Bug note deleted');
      await refresh();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Delete failed');
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Error Center</h1>
          <p className="text-sm text-muted-foreground">
            SHA-256 deduplicated error memory groups, triage workflow, and operator bug notes.
          </p>
        </div>
        <Button variant="outline" onClick={() => void refresh()}>
          <RefreshCw className="mr-2 h-4 w-4" />
          Refresh
        </Button>
      </div>

      <Tabs defaultValue="errors">
        <TabsList>
          <TabsTrigger value="errors">Error Memory ({errors.length})</TabsTrigger>
          <TabsTrigger value="bugs">Bug Notes ({bugNotes.length})</TabsTrigger>
        </TabsList>

        <TabsContent value="errors" className="space-y-4">
          <div className="flex gap-2">
            {['all', 'new', 'acknowledged', 'resolved'].map((st) => (
              <Button
                key={st}
                size="sm"
                variant={statusFilter === st ? 'default' : 'outline'}
                onClick={() => setStatusFilter(st)}
              >
                {st.toUpperCase()}
              </Button>
            ))}
          </div>

          <Card>
            <CardContent className="pt-6 space-y-3">
              {errors.length === 0 ? (
                <p className="text-sm text-muted-foreground">No recorded error groups match this filter.</p>
              ) : (
                errors.map((err) => (
                  <div key={err.id} className="flex flex-wrap items-center justify-between gap-3 rounded border p-3 text-sm">
                    <div className="space-y-1">
                      <div className="flex items-center gap-2 font-medium">
                        <AlertCircle className="h-4 w-4 text-destructive" />
                        <span>{err.errorMessage}</span>
                        <Badge variant="secondary">&times;{err.occurrenceCount}</Badge>
                        <Badge variant="outline">{err.status}</Badge>
                      </div>
                      <div className="text-xs text-muted-foreground">
                        Module: {err.moduleName || 'core'} • Command: {err.commandName || 'n/a'} • Last seen: {err.lastSeenAt}
                      </div>
                    </div>
                    <div className="flex gap-2">
                      {err.status !== 'acknowledged' && (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => void handleUpdateErrorStatus(err.id, 'acknowledged')}
                        >
                          Acknowledge
                        </Button>
                      )}
                      {err.status !== 'resolved' && (
                        <Button
                          size="sm"
                          onClick={() => void handleUpdateErrorStatus(err.id, 'resolved')}
                        >
                          <CheckCircle2 className="mr-1 h-3 w-3" />
                          Resolve
                        </Button>
                      )}
                    </div>
                  </div>
                ))
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="bugs" className="space-y-4">
          <div className="grid gap-4 lg:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle>Log Bug Note</CardTitle>
                <CardDescription>Track reproduction steps or environment anomalies locally.</CardDescription>
              </CardHeader>
              <CardContent>
                <form onSubmit={(e) => void handleCreateBugNote(e)} className="space-y-3">
                  <div>
                    <Label htmlFor="bug-title">Title</Label>
                    <Input id="bug-title" value={bugTitle} onChange={(e) => setBugTitle(e.target.value)} required />
                  </div>
                  <div>
                    <Label htmlFor="bug-desc">Description</Label>
                    <Textarea
                      id="bug-desc"
                      value={bugDesc}
                      onChange={(e) => setBugDesc(e.target.value)}
                      rows={3}
                    />
                  </div>
                  <Button type="submit">
                    <Plus className="mr-2 h-4 w-4" />
                    Save Bug Note
                  </Button>
                </form>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Tracked Notes ({bugNotes.length})</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                {bugNotes.length === 0 ? (
                  <p className="text-sm text-muted-foreground">No bug notes logged yet.</p>
                ) : (
                  bugNotes.map((n) => (
                    <div key={n.id} className="flex items-start justify-between gap-2 rounded border p-3 text-sm">
                      <div>
                        <div className="flex items-center gap-2 font-medium">
                          <Bug className="h-4 w-4 text-muted-foreground" />
                          {n.title}
                          <Badge variant={n.status === 'SOLVED' ? 'outline' : 'secondary'}>{n.status}</Badge>
                        </div>
                        {n.description && <p className="mt-1 text-xs text-muted-foreground">{n.description}</p>}
                      </div>
                      <div className="flex gap-1">
                        <Button size="sm" variant="outline" onClick={() => void handleToggleBugNote(n)}>
                          {n.status === 'OPEN' ? 'Mark Solved' : 'Reopen'}
                        </Button>
                        <Button size="sm" variant="ghost" onClick={() => void handleDeleteBugNote(n.id)}>
                          <Trash2 className="h-4 w-4 text-destructive" />
                        </Button>
                      </div>
                    </div>
                  ))
                )}
              </CardContent>
            </Card>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}
