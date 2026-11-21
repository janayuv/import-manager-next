import * as React from 'react';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Label } from '@/components/ui/label';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Switch } from '@/components/ui/switch';
import { Building2, Landmark, MapPin, ShieldCheck, Sparkles } from 'lucide-react';
import { useCompany } from '@/contexts/CompanyContext';
import type { CreateCompanyInput } from '@/types/company';

interface CreateCompanyModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function CreateCompanyModal({ open, onOpenChange }: CreateCompanyModalProps) {
  const { createCompany } = useCompany();
  const [submitting, setSubmitting] = React.useState(false);
  const [switchImmediately, setSwitchImmediately] = React.useState(true);

  // Form state
  const [name, setName] = React.useState('');
  const [shortCode, setShortCode] = React.useState('');
  const [baseCurrency, setBaseCurrency] = React.useState('INR');
  const [iecCode, setIecCode] = React.useState('');
  const [gstin, setGstin] = React.useState('');
  const [pan, setPan] = React.useState('');
  const [phone, setPhone] = React.useState('');
  const [email, setEmail] = React.useState('');
  const [website, setWebsite] = React.useState('');

  const [addressLine1, setAddressLine1] = React.useState('');
  const [addressLine2, setAddressLine2] = React.useState('');
  const [city, setCity] = React.useState('');
  const [state, setState] = React.useState('');
  const [postalCode, setPostalCode] = React.useState('');
  const [country, setCountry] = React.useState('India');

  const [beneficiaryName, setBeneficiaryName] = React.useState('');
  const [bankName, setBankName] = React.useState('');
  const [accountNumber, setAccountNumber] = React.useState('');
  const [ifscCode, setIfscCode] = React.useState('');
  const [swiftCode, setSwiftCode] = React.useState('');
  const [bankBranch, setBankBranch] = React.useState('');
  const [bankAddress, setBankAddress] = React.useState('');

  // Auto-generate short code when name changes (if short code hasn't been manually customized)
  const handleNameChange = (val: string) => {
    setName(val);
    if (!shortCode || shortCode.length <= 4) {
      const clean = val.replace(/[^a-zA-Z0-9]/g, '').slice(0, 4).toUpperCase();
      if (clean) setShortCode(clean);
    }
  };

  const handleReset = () => {
    setName('');
    setShortCode('');
    setBaseCurrency('INR');
    setIecCode('');
    setGstin('');
    setPan('');
    setPhone('');
    setEmail('');
    setWebsite('');
    setAddressLine1('');
    setAddressLine2('');
    setCity('');
    setState('');
    setPostalCode('');
    setCountry('India');
    setBeneficiaryName('');
    setBankName('');
    setAccountNumber('');
    setIfscCode('');
    setSwiftCode('');
    setBankBranch('');
    setBankAddress('');
    setSwitchImmediately(true);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;

    setSubmitting(true);
    try {
      const payload: CreateCompanyInput = {
        name: name.trim(),
        shortCode: shortCode.trim().toUpperCase(),
        baseCurrency: baseCurrency.trim() || 'INR',
        financialYearFrom: '04-01',
        financialYearTo: '03-31',
        iecCode: iecCode.trim(),
        gstin: gstin.trim().toUpperCase(),
        pan: pan.trim().toUpperCase(),
        phone: phone.trim(),
        email: email.trim(),
        website: website.trim(),
        addressLine1: addressLine1.trim(),
        addressLine2: addressLine2.trim(),
        city: city.trim(),
        state: state.trim(),
        postalCode: postalCode.trim(),
        country: country.trim() || 'India',
        beneficiaryName: beneficiaryName.trim() || name.trim(),
        bankName: bankName.trim(),
        accountNumber: accountNumber.trim(),
        ifscCode: ifscCode.trim().toUpperCase(),
        swiftCode: swiftCode.trim().toUpperCase(),
        bankBranch: bankBranch.trim(),
        bankAddress: bankAddress.trim(),
        switchImmediately,
      };

      await createCompany(payload);
      handleReset();
      onOpenChange(false);
    } catch {
      // toast shown in context
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <div className="flex items-center gap-2">
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Building2 className="h-5 w-5" />
            </div>
            <div>
              <DialogTitle className="text-base font-semibold">Create New Company</DialogTitle>
              <DialogDescription className="text-xs text-muted-foreground">
                Provisions a dedicated encrypted SQLite database with clean, isolated tables for this enterprise.
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4">
          <Tabs defaultValue="general" className="w-full">
            <TabsList className="grid w-full grid-cols-4">
              <TabsTrigger value="general" className="gap-1.5 text-xs">
                <Building2 className="h-3.5 w-3.5" />
                <span>General</span>
              </TabsTrigger>
              <TabsTrigger value="tax" className="gap-1.5 text-xs">
                <ShieldCheck className="h-3.5 w-3.5" />
                <span>Tax &amp; IEC</span>
              </TabsTrigger>
              <TabsTrigger value="address" className="gap-1.5 text-xs">
                <MapPin className="h-3.5 w-3.5" />
                <span>Address</span>
              </TabsTrigger>
              <TabsTrigger value="banking" className="gap-1.5 text-xs">
                <Landmark className="h-3.5 w-3.5" />
                <span>Banking</span>
              </TabsTrigger>
            </TabsList>

            {/* TAB 1: GENERAL */}
            <TabsContent value="general" className="space-y-3 pt-3">
              <div className="space-y-1.5">
                <Label htmlFor="company-name" className="text-xs font-medium">
                  Company Legal Name <span className="text-destructive">*</span>
                </Label>
                <Input
                  id="company-name"
                  placeholder="e.g. Apex Global Trading Pvt Ltd"
                  value={name}
                  onChange={(e) => handleNameChange(e.target.value)}
                  required
                  autoFocus
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="short-code" className="text-xs font-medium">
                    Short Code / Identifier <span className="text-destructive">*</span>
                  </Label>
                  <Input
                    id="short-code"
                    placeholder="e.g. APEX (2-12 uppercase chars)"
                    value={shortCode}
                    onChange={(e) => setShortCode(e.target.value.toUpperCase())}
                    maxLength={12}
                    required
                  />
                  <p className="text-[11px] text-muted-foreground">Used as badge &amp; document prefix tag</p>
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="base-currency" className="text-xs font-medium">
                    Base Currency
                  </Label>
                  <Input
                    id="base-currency"
                    placeholder="e.g. INR"
                    value={baseCurrency}
                    onChange={(e) => setBaseCurrency(e.target.value.toUpperCase())}
                  />
                  <p className="text-[11px] text-muted-foreground">Reporting currency (default: INR)</p>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="company-email" className="text-xs font-medium">
                    Contact Email
                  </Label>
                  <Input
                    id="company-email"
                    type="email"
                    placeholder="accounts@apexglobal.com"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="company-phone" className="text-xs font-medium">
                    Contact Phone
                  </Label>
                  <Input
                    id="company-phone"
                    placeholder="+91 22 1234 5678"
                    value={phone}
                    onChange={(e) => setPhone(e.target.value)}
                  />
                </div>
              </div>
            </TabsContent>

            {/* TAB 2: TAX & REGISTRATIONS */}
            <TabsContent value="tax" className="space-y-3 pt-3">
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="iec-code" className="text-xs font-medium">
                    IEC (Import Export Code)
                  </Label>
                  <Input
                    id="iec-code"
                    placeholder="10-digit DGFT IEC Code"
                    value={iecCode}
                    onChange={(e) => setIecCode(e.target.value)}
                    maxLength={10}
                  />
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="gstin" className="text-xs font-medium">
                    GSTIN
                  </Label>
                  <Input
                    id="gstin"
                    placeholder="15-character GSTIN"
                    value={gstin}
                    onChange={(e) => setGstin(e.target.value.toUpperCase())}
                    maxLength={15}
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="pan" className="text-xs font-medium">
                    Permanent Account Number (PAN)
                  </Label>
                  <Input
                    id="pan"
                    placeholder="10-digit PAN"
                    value={pan}
                    onChange={(e) => setPan(e.target.value.toUpperCase())}
                    maxLength={10}
                  />
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="website" className="text-xs font-medium">
                    Company Website
                  </Label>
                  <Input
                    id="website"
                    placeholder="https://www.apexglobal.com"
                    value={website}
                    onChange={(e) => setWebsite(e.target.value)}
                  />
                </div>
              </div>
            </TabsContent>

            {/* TAB 3: REGISTERED ADDRESS */}
            <TabsContent value="address" className="space-y-3 pt-3">
              <div className="space-y-1.5">
                <Label htmlFor="address-1" className="text-xs font-medium">
                  Address Line 1
                </Label>
                <Textarea
                  id="address-1"
                  placeholder="Plot / Office / Street address"
                  value={addressLine1}
                  onChange={(e) => setAddressLine1(e.target.value)}
                  rows={2}
                />
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="address-2" className="text-xs font-medium">
                  Address Line 2 (Optional)
                </Label>
                <Input
                  id="address-2"
                  placeholder="Industrial Area / Landmark"
                  value={addressLine2}
                  onChange={(e) => setAddressLine2(e.target.value)}
                />
              </div>

              <div className="grid grid-cols-3 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="city" className="text-xs font-medium">
                    City
                  </Label>
                  <Input
                    id="city"
                    placeholder="Mumbai"
                    value={city}
                    onChange={(e) => setCity(e.target.value)}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="state" className="text-xs font-medium">
                    State
                  </Label>
                  <Input
                    id="state"
                    placeholder="Maharashtra"
                    value={state}
                    onChange={(e) => setState(e.target.value)}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="postal-code" className="text-xs font-medium">
                    PIN / Postal Code
                  </Label>
                  <Input
                    id="postal-code"
                    placeholder="400001"
                    value={postalCode}
                    onChange={(e) => setPostalCode(e.target.value)}
                  />
                </div>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="country" className="text-xs font-medium">
                  Country
                </Label>
                <Input
                  id="country"
                  placeholder="India"
                  value={country}
                  onChange={(e) => setCountry(e.target.value)}
                />
              </div>
            </TabsContent>

            {/* TAB 4: BANKING & SWIFT */}
            <TabsContent value="banking" className="space-y-3 pt-3">
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="beneficiary-name" className="text-xs font-medium">
                    Beneficiary Name
                  </Label>
                  <Input
                    id="beneficiary-name"
                    placeholder="Name as registered with bank"
                    value={beneficiaryName}
                    onChange={(e) => setBeneficiaryName(e.target.value)}
                  />
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="bank-name" className="text-xs font-medium">
                    Bank Name
                  </Label>
                  <Input
                    id="bank-name"
                    placeholder="e.g. State Bank of India / HDFC"
                    value={bankName}
                    onChange={(e) => setBankName(e.target.value)}
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="account-num" className="text-xs font-medium">
                    Bank Account Number
                  </Label>
                  <Input
                    id="account-num"
                    placeholder="Current / Forex Account No."
                    value={accountNumber}
                    onChange={(e) => setAccountNumber(e.target.value)}
                  />
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="ifsc-code" className="text-xs font-medium">
                    IFSC Code
                  </Label>
                  <Input
                    id="ifsc-code"
                    placeholder="e.g. HDFC0001234"
                    value={ifscCode}
                    onChange={(e) => setIfscCode(e.target.value.toUpperCase())}
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="swift-code" className="text-xs font-medium">
                    SWIFT / BIC Code (for FX Remittance)
                  </Label>
                  <Input
                    id="swift-code"
                    placeholder="e.g. HDFCINBB"
                    value={swiftCode}
                    onChange={(e) => setSwiftCode(e.target.value.toUpperCase())}
                  />
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="bank-branch" className="text-xs font-medium">
                    Branch Name
                  </Label>
                  <Input
                    id="bank-branch"
                    placeholder="Fort Branch, Mumbai"
                    value={bankBranch}
                    onChange={(e) => setBankBranch(e.target.value)}
                  />
                </div>
              </div>
            </TabsContent>
          </Tabs>

          <div className="flex items-center justify-between rounded-lg border border-border bg-muted/40 p-3">
            <div className="space-y-0.5">
              <span className="text-xs font-medium text-foreground">Switch to this company immediately</span>
              <p className="text-[11px] text-muted-foreground">
                Mounts the new encrypted database as the active company workspace upon creation.
              </p>
            </div>
            <Switch checked={switchImmediately} onCheckedChange={setSwitchImmediately} />
          </div>

          <DialogFooter className="gap-2 sm:gap-0">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => onOpenChange(false)}
              disabled={submitting}
            >
              Cancel
            </Button>
            <Button type="submit" size="sm" disabled={submitting || !name.trim()} className="gap-1.5">
              {submitting ? (
                <>
                  <div className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-primary-foreground border-t-transparent" />
                  <span>Provisioning Database...</span>
                </>
              ) : (
                <>
                  <Sparkles className="h-3.5 w-3.5" />
                  <span>Create Company &amp; Database</span>
                </>
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
