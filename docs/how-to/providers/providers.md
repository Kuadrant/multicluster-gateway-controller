# DNS Providers supported by multi-cluster gateway cluster

## Introduction
The following document tells you everything you need to know about the DNS Provider the multi cluster gateway cluster supports and the specific features we offer that utilises them.

## Current provider supported

In the current iteration of the multi-cluster gateway controller we support both **AWS (Amazon web services) Route 53** and **GCP (Google cloud provider) cloud DNS**. 

### Geolocation

Geolocation is a feature available in both DNS providers we support. A location is needed for both DNS Providers, please see below for the supported location for the provider you require.

**Note:** 
If a unsupported value is given to a provider, DNS records will **not** be created. Please choose carefully. For more information of what location is right for your needs please read said providers documentation. 

#### Locations supported per DNS provider

| Supported     | AWS | GCP |
|---------------|-----|-----|
| Continents    | :white_check_mark: |  :x: |
| Country codes | :white_check_mark: |  :x:  |
| States        | :white_check_mark: |  :x:  |
| Regions       |  :x:  | :white_check_mark: |  

#### Continents and country codes supported by AWS route 53

**:NOTE:** For more information please the official AWS documentation 

| Continents    | Country codes                                                                                                                                                                                                                             |
|---------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Africa        | AO, BF, BI, BJ, BW, CD, CF, CG, CI, CM, CV, DJ, DZ, EG, ER, ET, GA, GH,  GM, GN, GQ, GW, KE, KM,  LR, LS, LY, MA, MG, ML,  MR, MU, MW, MZ, NA, NE,  NG, RE, RW, SC, SD, SH,  SL, SN, SO, SS, ST, SZ,  TD, TG, TN, TZ, UG, YT,  ZA, ZM, ZW |
| Antarctica    | AQ, GS, TF                                                                                                                                                                                                                                |
| Asia          | AE, AF, AM, AZ, BD, BH,  BN, BT, CC, CN, GE, HK,  ID, IL, IN, IO, IQ, IR,  JO, JP, KG, KH, KP, KR,  KW, KZ, LA, LB, LK, MM,  MN, MO, MV, MY, NP, OM,  PH, PK, PS, QA, SA, SG,  SY, TH, TJ, TM, TW, UZ,  VN, YE                            |
| Europe        | AD, AL, AT, AX, BA, BE,  BG, BY, CH, CY, CZ, DE,  DK, EE, ES, FI, FO, FR,  GB, GG, GI, GR, HR, HU,  IE, IM, IS, IT, JE, LI,  LT, LU, LV, MC, MD, ME,  MK, MT, NL, NO, PL, PT,  RO, RS, RU, SE, SI, SJ,  SK, SM, TR, UA, VA, XK            |
| North America | AG, AI, AW, BB, BL, BM,  BQ, BS, BZ, CA, CR, CU,  CW, DM, DO, GD, GL, GP,  GT, HN, HT, JM, KN, KY,  LC, MF, MQ, MS, MX, NI,  PA, PM, PR, SV, SX, TC,  TT, US, VC, VG, VI                                                                  |
| Oceania       | AS, AU, CK, FJ, FM, GU,  KI, MH, MP, NC, NF, NR,  NU, NZ, PF, PG, PN, PW,  SB, TK, TL, TO, TV, UM,  VU, WF, WS                                                                                                                            |
| South America | AR, BO, BR, CL, CO, EC,  FK, GF, GY, PE, PY, SR,  UY, VE                                                                                                                                                                                  |
#### Regions supported by GCP CLoud DNS

| asia-east1-a, asia-east1-c, asia-east2-b, asia-northeast1-a, asia-northeast1-c, asia-northeast2-b, asia-northeast3-a, asia-northeast3-c, asia-south1-b, asia-south2-a, asia-south2-c, asia-southeast1-b, asia-southeast2-a, asia-southeast2-c, | australia-southeast1-b, australia-southeast2-a, australia-southeast2-c, | europe-central2-b, europe-north1-a, europe-north1-c, europe-southwest1-b, europe-west1-b, europe-west1-d, europe-west12-b, europe-west2-a, europe-west2-c, europe-west3-b, europe-west4-a, europe-west4-c, europe-west6-b, europe-west8-a, europe-west8-c, europe-west9-b, | me-central1-a, me-central1-c, me-west1-b, | northamerica-northeast1-a, northamerica-northeast1-c, northamerica-northeast2-b, | southamerica-east1-a, southamerica-east1-c, southamerica-west1-b, | us-central1-a, us-central1-c, us-east1-b, us-east1-d, us-east4-b, us-east5-a, us-east5-c, us-south1-b, us-west1-a, us-west1-c, us-west2-b, us-west3-a, us-west3-c, us-west4-b, |
|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------|----------------------------------------------------------------------------------|-------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|

