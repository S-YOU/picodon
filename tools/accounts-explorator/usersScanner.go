package accounts_explorator

import (
	"fmt"
	"github.com/sapiens-sapide/go-mastodon"
	"log"
	"time"
)

// worker that continuously goes through accounts in db
// to retreive accounts' relationships of an instance and
// save new discovered users and instances.
func (iw *InstanceWorker) ScanUsers() {
	fmt.Printf("starting ScanUsers worker for %s\n", iw.Instance.Domain)
	c := mastodon.NewClient(&mastodon.Config{
		Server:       "https://" + iw.Instance.Domain,
		ClientID:     iw.Instance.APIid,
		ClientSecret: iw.Instance.APIsecret,
	})
	for {
		err := c.Authenticate(iw.Context, iw.Instance.Username, iw.Instance.Password)
		if err != nil {
			log.Printf("[ScanInstanceUsers] : auth against instance %s failed with error : %s", iw.Instance.Domain, err)
		}
		accounts, err := iw.Backend.FindAccountsToScan(&(iw.Instance))
		for _, account := range accounts {
			followers, err := c.GetAccountFollowers(iw.Context, int64(account.ID))
			if err != nil {
				log.Printf("[ScanInstanceUsers] error when getting followers for account %d@%s : %s", account.ID, iw.Instance.Domain, err)
				time.Sleep(2 * time.Second) // to prevent throttling

			} else {
				account.LocalFollowers, account.RemoteFollowers = iw.iterateAccounts(account.ID, followers)
			}
			followings, err := c.GetAccountFollowing(iw.Context, int64(account.ID))
			if err != nil {
				log.Printf("[ScanInstanceUsers] error when getting followings for account %d@%s : %s", account.ID, iw.Instance.Domain, err)
				time.Sleep(2 * time.Second) // to prevent throttling
			} else {
				account.LocalFollowings, account.RemoteFollowings = iw.iterateAccounts(account.ID, followings)
			}
			if err == nil {
				account.LastScan = time.Now()
				iw.Backend.SaveAccount(account)
			}
		}

		time.Sleep(5 * time.Minute)
	}
}

func (iw *InstanceWorker) iterateAccounts(accountID uint, accts []*mastodon.Account) (local, remote uint) {
	for _, mastodonAcct := range accts {
		user, instance, err := splitUserAndInstance(mastodonAcct.Acct, iw.Instance.Domain)
		if err != nil {
			fmt.Printf("error : %s\n", err)
			continue
		}
		acct := Account{
			Username: user,
			Instance: instance,
		}
		if instance != iw.Instance.Domain {
			id, err := GetRemoteAccountID(user, instance)
			if err == nil {
				acct.ID = uint(id)
			}
		} else {
			acct.ID = uint(mastodonAcct.ID)
		}
		if acct.ID != 0 {
			iw.Backend.CreateAccountIfNotExist(acct)
		}
		iw.Backend.CreateInstanceIfNotExist(Instance{Domain: instance})
		if instance == iw.Instance.Domain {
			local++
		} else {
			remote++
		}
	}
	return
}
