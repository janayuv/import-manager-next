import * as React from 'react';
import { Palette, RotateCcw, Save, Settings2 } from 'lucide-react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useAuth } from '@/contexts/AuthContext';
import {
  applyThemeProfile,
  settingsService,
  type AppSettingsProfile,
} from '@/services/dashboard-settings-service';

const MODULE_LABELS: Record<string, string> = {
  shipment: 'Shipments',
  invoice: 'Invoices',
  boe: 'BOE Registry',
  boeSummary: 'BOE Summary',
  supplier: 'Suppliers',
  itemMaster: 'Item Master',
  expenses: 'Expenses',
};

export function SettingsPage() {
  const { hasPermission } = useAuth();
  const canWrite = hasPermission('data.write');

  const [profile, setProfile] = React.useState<AppSettingsProfile | null>(null);
  const [selectedModule, setSelectedModule] = React.useState('shipment');
  const [saving, setSaving] = React.useState(false);

  React.useEffect(() => {
    void settingsService
      .getProfile()
      .then((p) => {
        setProfile(p);
        applyThemeProfile(p);
      })
      .catch((err) => toast.error(err instanceof Error ? err.message : String(err)));
  }, []);

  const handleSave = async () => {
    if (!profile) return;
    setSaving(true);
    try {
      const saved = await settingsService.saveProfile(profile);
      setProfile(saved);
      applyThemeProfile(saved);
      toast.success('Application settings saved');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  const handleResetModule = async () => {
    try {
      const next = await settingsService.resetModuleDefaults(selectedModule);
      setProfile(next);
      toast.success(`Reset ${MODULE_LABELS[selectedModule] ?? selectedModule} defaults`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  if (!profile) {
    return (
      <div className="text-sm text-muted-foreground">Loading application settings...</div>
    );
  }

  const modCfg = profile.modules[selectedModule] ?? {
    visibleFields: [],
    itemsPerPage: 10,
    decimalPrecision: 2,
    dateFormat: profile.dateFormat,
    fieldCases: {},
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight">
            <Settings2 className="h-6 w-6 text-primary" />
            Application Settings &amp; Formatting
          </h1>
          <p className="text-sm text-muted-foreground">
            Theme preset, dark/light mode, invoice precision, regional formatting, and module table preferences.
          </p>
        </div>
        {canWrite && (
          <Button disabled={saving} onClick={() => void handleSave()}>
            <Save className="h-4 w-4" />
            {saving ? 'Saving...' : 'Save Settings'}
          </Button>
        )}
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <Palette className="h-4 w-4 text-primary" />
              Appearance &amp; Theme
            </CardTitle>
            <CardDescription>Schema v{profile.schemaVersion}</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1">
              <Label>Theme Preset</Label>
              <Select
                value={profile.theme}
                onValueChange={(v) => {
                  const next = { ...profile, theme: v };
                  setProfile(next);
                  applyThemeProfile(next);
                }}
              >
                <SelectTrigger aria-label="Theme Preset">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="default">Default ERP Slate</SelectItem>
                  <SelectItem value="steel">Industrial Steel</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1">
              <Label>Color Mode</Label>
              <Select
                value={profile.mode}
                onValueChange={(v) => {
                  const next = { ...profile, mode: v };
                  setProfile(next);
                  applyThemeProfile(next);
                }}
              >
                <SelectTrigger aria-label="Color Mode">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="system">System</SelectItem>
                  <SelectItem value="light">Light</SelectItem>
                  <SelectItem value="dark">Dark</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Invoice Precision &amp; Formatting</CardTitle>
            <CardDescription>Decimal precision restricted to 0 or 2 places per business rules</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1">
              <Label>Line Total Decimals</Label>
              <Select
                value={String(profile.lineTotalDecimals)}
                onValueChange={(v) =>
                  setProfile({ ...profile, lineTotalDecimals: Number.parseInt(v, 10) })
                }
              >
                <SelectTrigger aria-label="Line Total Decimals">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="2">2 Decimals (Standard)</SelectItem>
                  <SelectItem value="0">0 Decimals (Rounded)</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1">
              <Label>Invoice Total Decimals</Label>
              <Select
                value={String(profile.invoiceTotalDecimals)}
                onValueChange={(v) =>
                  setProfile({ ...profile, invoiceTotalDecimals: Number.parseInt(v, 10) })
                }
              >
                <SelectTrigger aria-label="Invoice Total Decimals">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="2">2 Decimals (Standard)</SelectItem>
                  <SelectItem value="0">0 Decimals (Rounded)</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1">
              <Label>Date Display Format</Label>
              <Select
                value={profile.dateFormat}
                onValueChange={(v) => setProfile({ ...profile, dateFormat: v })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="DD/MM/YYYY">DD/MM/YYYY (Default)</SelectItem>
                  <SelectItem value="YYYY-MM-DD">YYYY-MM-DD (ISO)</SelectItem>
                  <SelectItem value="MM/DD/YYYY">MM/DD/YYYY</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1">
              <Label>Number Grouping</Label>
              <Select
                value={profile.numberFormat}
                onValueChange={(v) => setProfile({ ...profile, numberFormat: v })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="en-IN">Indian (1,00,000.00)</SelectItem>
                  <SelectItem value="en-US">International (100,000.00)</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle className="text-base">Per-Module Table Preferences (7 Modules)</CardTitle>
            <CardDescription>
              Pagination size and formatting defaults per ERP view.
            </CardDescription>
          </div>
          {canWrite && (
            <Button variant="outline" size="sm" onClick={() => void handleResetModule()}>
              <RotateCcw className="h-3.5 w-3.5" />
              Reset Module Defaults
            </Button>
          )}
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-3">
            <div className="space-y-1">
              <Label>Module</Label>
              <Select value={selectedModule} onValueChange={setSelectedModule}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {Object.entries(MODULE_LABELS).map(([k, lbl]) => (
                    <SelectItem key={k} value={k}>
                      {lbl}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1">
              <Label>Rows Per Page (5–200)</Label>
              <Input
                type="number"
                min={5}
                max={200}
                value={modCfg.itemsPerPage}
                onChange={(e) => {
                  const nextVal = Number.parseInt(e.target.value, 10) || 10;
                  setProfile({
                    ...profile,
                    modules: {
                      ...profile.modules,
                      [selectedModule]: { ...modCfg, itemsPerPage: nextVal },
                    },
                  });
                }}
              />
            </div>

            <div className="flex items-end pb-2">
              {selectedModule === 'shipment' && (
                <Badge variant="outline">
                  shipmentType case locked: {modCfg.fieldCases?.shipmentType ?? 'uppercase'}
                </Badge>
              )}
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
