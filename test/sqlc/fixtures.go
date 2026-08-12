package sqlc

// DiscountType models a string-backed sqlc enum.
type DiscountType string

const (
	DiscountTypePercentage DiscountType = "percentage"
	DiscountTypeFixed      DiscountType = "fixed"
)

// CouponScope models a second sqlc enum used by Coupon.
type CouponScope string

const (
	CouponScopeCart CouponScope = "cart"
	CouponScopeItem CouponScope = "item"
)

// Coupon exercises enum fields on sqlc-style rows.
type Coupon struct {
	Type  DiscountType `json:"type"`
	Scope CouponScope  `json:"scope"`
}

// OrderDeliveryStatus models a nullable sqlc enum wrapper.
type OrderDeliveryStatus string

const (
	OrderDeliveryStatusPending OrderDeliveryStatus = "pending"
	OrderDeliveryStatusSent    OrderDeliveryStatus = "sent"
)

// NullOrderDeliveryStatus follows sqlc's nullable enum wrapper shape.
type NullOrderDeliveryStatus struct {
	OrderDeliveryStatus OrderDeliveryStatus `json:"order_delivery_status"`
	Valid               bool                `json:"valid"`
}

// ListOrdersRow is a response row used in annotation-based tests.
type ListOrdersRow struct {
	ID int64 `json:"id"`
}
