import * as React from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { AlertTriangle, KeyRound, Lock, ShieldCheck, Wrench } from 'lucide-react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useAuth } from '@/contexts/AuthContext';
import { securityService } from '@/services/security-service';

const setupSchema = z
  .object({
    username: z.string().trim().min(3, 'Username must be at least 3 characters'),
    password: z
      .string()
      .min(8, 'Password must be at least 8 characters')
      .regex(/[A-Z]/, 'Include at least one uppercase letter')
      .regex(/[a-z]/, 'Include at least one lowercase letter')
      .regex(/[0-9]/, 'Include at least one digit')
      .regex(/[^A-Za-z0-9]/, 'Include at least one special character'),
    confirmPassword: z.string(),
  })
  .refine((vals) => vals.password === vals.confirmPassword, {
    message: 'Passwords do not match',
    path: ['confirmPassword'],
  });

const loginSchema = z.object({
  username: z.string().trim().min(1, 'Username is required'),
  password: z.string().min(1, 'Password is required'),
  rememberMe: z.boolean(),
});

const recoveryPasswordSchema = z.object({
  newPassword: z
    .string()
    .min(8, 'Password must be at least 8 characters')
    .regex(/[A-Z]/, 'Include at least one uppercase letter')
    .regex(/[a-z]/, 'Include at least one lowercase letter')
    .regex(/[0-9]/, 'Include at least one digit'),
});

type SetupFormValues = z.infer<typeof setupSchema>;
type LoginFormValues = z.infer<typeof loginSchema>;
type RecoveryFormValues = z.infer<typeof recoveryPasswordSchema>;

export function LoginPage() {
  const { setupStatus, login, completeSetup, refreshAuth } = useAuth();
  const [authError, setAuthError] = React.useState<string | null>(null);
  const [showRecoveryPanel, setShowRecoveryPanel] = React.useState(false);

  const isSetup = setupStatus?.needsSetup ?? false;
  const recoveryMode = setupStatus?.recoveryMode ?? false;

  const setupForm = useForm<SetupFormValues>({
    resolver: zodResolver(setupSchema),
    defaultValues: { username: 'admin', password: '', confirmPassword: '' },
  });

  const loginForm = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: {
      username: setupStatus?.adminUsername || 'admin',
      password: '',
      rememberMe: false,
    },
  });

  const recoveryForm = useForm<RecoveryFormValues>({
    resolver: zodResolver(recoveryPasswordSchema),
    defaultValues: { newPassword: '' },
  });

  React.useEffect(() => {
    if (setupStatus?.adminUsername) {
      loginForm.setValue('username', setupStatus.adminUsername);
    }
  }, [setupStatus?.adminUsername, loginForm]);

  const onSubmitSetup = async (values: SetupFormValues) => {
    setAuthError(null);
    try {
      await completeSetup(values.username, values.password);
      toast.success('Administrator account created');
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setAuthError(msg);
      toast.error(msg);
    }
  };

  const onSubmitLogin = async (values: LoginFormValues) => {
    setAuthError(null);
    try {
      await login(values.username, values.password, values.rememberMe);
      toast.success('Signed in');
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setAuthError(msg);
    }
  };

  const handleRecoveryClearLockout = async () => {
    try {
      await securityService.recoveryClearLockout(setupStatus?.adminUsername || 'admin');
      toast.success('Lockout state cleared');
      setAuthError(null);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleRecoveryResetPolicy = async () => {
    try {
      await securityService.recoveryResetSecurityPolicy();
      toast.success('Security policy reset to factory defaults');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleRecoverySetPassword = async (values: RecoveryFormValues) => {
    try {
      await securityService.recoverySetAdminPassword(values.newPassword);
      recoveryForm.reset();
      await refreshAuth();
      toast.success('Admin password updated via recovery mode');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-6">
      <div className="w-full max-w-md space-y-4">
        <div className="text-center">
          <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10 text-primary">
            {isSetup ? <KeyRound className="h-6 w-6" /> : <Lock className="h-6 w-6" />}
          </div>
          <h1 className="text-2xl font-bold tracking-tight">Import Manager</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {isSetup
              ? 'Create the local administrator account to initialize the encrypted workspace.'
              : 'Sign in with your local administrator credentials.'}
          </p>
        </div>

        {recoveryMode && (
          <div className="rounded-lg border border-amber-500/50 bg-amber-500/10 p-3 text-sm text-amber-900 dark:text-amber-200">
            <div className="flex items-center justify-between gap-2">
              <div className="flex items-center gap-2 font-medium">
                <AlertTriangle className="h-4 w-4 shrink-0" />
                <span>Emergency Recovery Mode Active</span>
              </div>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => setShowRecoveryPanel((v) => !v)}
              >
                <Wrench className="h-3.5 w-3.5" />
                {showRecoveryPanel ? 'Hide Tools' : 'Recovery Tools'}
              </Button>
            </div>
          </div>
        )}

        {recoveryMode && showRecoveryPanel && (
          <Card className="border-amber-500/40">
            <CardHeader>
              <CardTitle className="text-base">Emergency Recovery Actions</CardTitle>
              <CardDescription>
                Available only because the application was launched with --recovery.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex flex-wrap gap-2">
                <Button type="button" variant="outline" size="sm" onClick={() => void handleRecoveryClearLockout()}>
                  Clear Account Lockout
                </Button>
                <Button type="button" variant="outline" size="sm" onClick={() => void handleRecoveryResetPolicy()}>
                  Reset Lockout &amp; Idle Policy
                </Button>
              </div>
              <form
                onSubmit={recoveryForm.handleSubmit(handleRecoverySetPassword)}
                className="space-y-2 border-t border-border pt-3"
              >
                <Label htmlFor="recovery-new-password">Reset Admin Password</Label>
                <div className="flex gap-2">
                  <Input
                    id="recovery-new-password"
                    type="password"
                    placeholder="New strong password"
                    {...recoveryForm.register('newPassword')}
                  />
                  <Button type="submit" size="sm" disabled={recoveryForm.formState.isSubmitting}>
                    Apply
                  </Button>
                </div>
                {recoveryForm.formState.errors.newPassword && (
                  <p className="text-xs text-destructive">
                    {recoveryForm.formState.errors.newPassword.message}
                  </p>
                )}
              </form>
            </CardContent>
          </Card>
        )}

        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>{isSetup ? 'Initial Administrator Setup' : 'Desktop Sign-In'}</CardTitle>
              <Badge variant="success">
                <ShieldCheck className="mr-1 h-3 w-3" />
                SQLCipher
              </Badge>
            </div>
            <CardDescription>
              {isSetup
                ? 'Passwords are hashed with Argon2id and never stored in plaintext.'
                : 'Single-user local Windows desktop session with backend RBAC enforcement.'}
            </CardDescription>
          </CardHeader>
          <CardContent>
            {authError && (
              <div
                role="alert"
                className="mb-4 rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive"
              >
                {authError}
              </div>
            )}

            {isSetup ? (
              <form onSubmit={setupForm.handleSubmit(onSubmitSetup)} className="space-y-4">
                <div className="space-y-1.5">
                  <Label htmlFor="setup-username">Administrator Username</Label>
                  <Input id="setup-username" autoComplete="username" {...setupForm.register('username')} />
                  {setupForm.formState.errors.username && (
                    <p className="text-xs text-destructive">{setupForm.formState.errors.username.message}</p>
                  )}
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="setup-password">Password</Label>
                  <Input
                    id="setup-password"
                    type="password"
                    autoComplete="new-password"
                    {...setupForm.register('password')}
                  />
                  {setupForm.formState.errors.password && (
                    <p className="text-xs text-destructive">{setupForm.formState.errors.password.message}</p>
                  )}
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="setup-confirm">Confirm Password</Label>
                  <Input
                    id="setup-confirm"
                    type="password"
                    autoComplete="new-password"
                    {...setupForm.register('confirmPassword')}
                  />
                  {setupForm.formState.errors.confirmPassword && (
                    <p className="text-xs text-destructive">
                      {setupForm.formState.errors.confirmPassword.message}
                    </p>
                  )}
                </div>

                <Button type="submit" className="w-full" disabled={setupForm.formState.isSubmitting}>
                  {setupForm.formState.isSubmitting ? 'Initializing...' : 'Create Administrator & Continue'}
                </Button>
              </form>
            ) : (
              <form onSubmit={loginForm.handleSubmit(onSubmitLogin)} className="space-y-4">
                <div className="space-y-1.5">
                  <Label htmlFor="login-username">Username</Label>
                  <Input id="login-username" autoComplete="username" {...loginForm.register('username')} />
                  {loginForm.formState.errors.username && (
                    <p className="text-xs text-destructive">{loginForm.formState.errors.username.message}</p>
                  )}
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="login-password">Password</Label>
                  <Input
                    id="login-password"
                    type="password"
                    autoComplete="current-password"
                    {...loginForm.register('password')}
                  />
                  {loginForm.formState.errors.password && (
                    <p className="text-xs text-destructive">{loginForm.formState.errors.password.message}</p>
                  )}
                </div>

                <label className="flex items-center gap-2 text-sm text-muted-foreground">
                  <input
                    type="checkbox"
                    className="h-4 w-4 rounded border-input"
                    {...loginForm.register('rememberMe')}
                  />
                  <span>Keep session active for 30 days on this Windows workstation</span>
                </label>

                <Button type="submit" className="w-full" disabled={loginForm.formState.isSubmitting}>
                  {loginForm.formState.isSubmitting ? 'Verifying...' : 'Sign In'}
                </Button>
              </form>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
