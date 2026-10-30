// Shorthand names for the types generated from the backend OpenAPI contract
// (`npm run api` regenerates schema.d.ts; never edit it by hand).
import type { components } from './schema'

type Schemas = components['schemas']

export type Me = Schemas['Me']
export type OrgSummary = Schemas['OrgSummary']
export type Organization = Schemas['Organization']
export type Role = Schemas['Role']
export type InviteRole = Schemas['InviteRole']
export type Invite = Schemas['Invite']
export type InviteList = Schemas['InviteList']
export type InvitePreview = Schemas['InvitePreview']
export type Membership = Schemas['Membership']
export type CreateOrgRequest = Schemas['CreateOrgRequest']
export type CreateInviteRequest = Schemas['CreateInviteRequest']
export type RegisterRequest = Schemas['RegisterRequest']
export type AccessTokenResponse = Schemas['AccessTokenResponse']
