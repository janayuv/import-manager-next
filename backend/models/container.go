package models

// ShipmentContainer represents a physical container or LCL cargo lot in a shipment.
type ShipmentContainer struct {
	ID                string  `json:"id"`
	ShipmentID        string  `json:"shipmentId"`
	ContainerNumber   string  `json:"containerNumber"`
	ContainerSizeType string  `json:"containerSizeType"` // 20FT, 40FT, 40FT_HC, 45FT, LCL, etc.
	SealNumber        string  `json:"sealNumber"`
	RouteType         string  `json:"routeType"`         // DIRECT, WAREHOUSE
	WarehouseName     string  `json:"warehouseName"`
	GateInDate        string  `json:"gateInDate"`
	GateOutDate       string  `json:"gateOutDate"`
	FreeDaysAllowed   int     `json:"freeDaysAllowed"`
	StorageRatePerDay float64 `json:"storageRatePerDay"`
	DeliveryStatus    string  `json:"deliveryStatus"`    // PENDING, IN_TRANSIT, IN_WAREHOUSE, DELIVERED
	DateOfDelivery    string  `json:"dateOfDelivery"`
	TransporterName   string  `json:"transporterName"`
	VehicleNumber     string  `json:"vehicleNumber"`
	LRNumber          string  `json:"lrNumber"`
	EmptyReturnDate   string  `json:"emptyReturnDate"`
	Remarks           string  `json:"remarks"`
	DwellDays         int     `json:"dwellDays"`         // Computed days in warehouse
	BillableDays      int     `json:"billableDays"`      // max(0, dwellDays - freeDaysAllowed)
	StorageCost       float64 `json:"storageCost"`       // billableDays * storageRatePerDay
	CreatedAt         string  `json:"createdAt"`
	UpdatedAt         string  `json:"updatedAt"`
}

// ShipmentContainerInput carries input fields for creating or updating a container.
type ShipmentContainerInput struct {
	ID                string  `json:"id,omitempty"`
	ShipmentID        string  `json:"shipmentId"`
	ContainerNumber   string  `json:"containerNumber"`
	ContainerSizeType string  `json:"containerSizeType"`
	SealNumber        string  `json:"sealNumber"`
	RouteType         string  `json:"routeType"`
	WarehouseName     string  `json:"warehouseName"`
	GateInDate        string  `json:"gateInDate"`
	GateOutDate       string  `json:"gateOutDate"`
	FreeDaysAllowed   int     `json:"freeDaysAllowed"`
	StorageRatePerDay float64 `json:"storageRatePerDay"`
	DeliveryStatus    string  `json:"deliveryStatus"`
	DateOfDelivery    string  `json:"dateOfDelivery"`
	TransporterName   string  `json:"transporterName"`
	VehicleNumber     string  `json:"vehicleNumber"`
	LRNumber          string  `json:"lrNumber"`
	EmptyReturnDate   string  `json:"emptyReturnDate"`
	Remarks           string  `json:"remarks"`
}

// ContainerMovementAction represents a batch or single operational action:
// "DIRECT_DELIVER", "MOVE_TO_WAREHOUSE", or "DISPATCH_FROM_WAREHOUSE".
type ContainerMovementAction struct {
	ShipmentID        string   `json:"shipmentId"`
	ContainerIDs      []string `json:"containerIds"`
	ActionType        string   `json:"actionType"` // DIRECT_DELIVER, MOVE_TO_WAREHOUSE, DISPATCH_FROM_WAREHOUSE
	WarehouseName     string   `json:"warehouseName,omitempty"`
	GateInDate        string   `json:"gateInDate,omitempty"`
	GateOutDate       string   `json:"gateOutDate,omitempty"`
	DateOfDelivery    string   `json:"dateOfDelivery,omitempty"`
	TransporterName   string   `json:"transporterName,omitempty"`
	VehicleNumber     string   `json:"vehicleNumber,omitempty"`
	LRNumber          string   `json:"lrNumber,omitempty"`
	EmptyReturnDate   string   `json:"emptyReturnDate,omitempty"`
	Remarks           string   `json:"remarks,omitempty"`
}

// WarehouseDwellSummary represents a container's warehouse storage details for reports and tracking.
type WarehouseDwellSummary struct {
	ContainerID       string  `json:"containerId"`
	ShipmentID        string  `json:"shipmentId"`
	SupplierName      string  `json:"supplierName"`
	InvoiceNumber     string  `json:"invoiceNumber"`
	BLAWBNumber       string  `json:"blAwbNumber"`
	ContainerNumber   string  `json:"containerNumber"`
	ContainerSizeType string  `json:"containerSizeType"`
	WarehouseName     string  `json:"warehouseName"`
	GateInDate        string  `json:"gateInDate"`
	GateOutDate       string  `json:"gateOutDate,omitempty"`
	DeliveryStatus    string  `json:"deliveryStatus"`
	DateOfDelivery    string  `json:"dateOfDelivery,omitempty"`
	DwellDays         int     `json:"dwellDays"`
	FreeDaysAllowed   int     `json:"freeDaysAllowed"`
	BillableDays      int     `json:"billableDays"`
	StorageRatePerDay float64 `json:"storageRatePerDay"`
	EstimatedCharges  float64 `json:"estimatedCharges"`
}
